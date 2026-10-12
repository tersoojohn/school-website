// Command server runs the school portal.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"schoolportal/internal/config"
	"schoolportal/internal/database"
	"schoolportal/internal/handlers"
	"schoolportal/internal/repository"
	"schoolportal/internal/routes"
	"schoolportal/internal/security"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	setupFor := flag.String("setup-link", "", "print a fresh one-time setup link for this username, then exit (also resets that account's password)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := database.Migrate(ctx, db); err != nil {
		return err
	}
	store := repository.New(db)

	if *setupFor != "" {
		return printSetupLink(ctx, store, cfg, *setupFor)
	}

	if err := ensurePrincipal(ctx, store, cfg, log); err != nil {
		return err
	}

	app, err := handlers.New(cfg, store, log)
	if err != nil {
		return err
	}
	handler, err := routes.New(app, store, cfg, log)
	if err != nil {
		return err
	}

	go func() { // clear expired sessions
		t := time.NewTicker(10 * time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := store.PurgeSessions(ctx, time.Now(), cfg.SessionIdle); err != nil {
					log.Error("purge sessions", "error", err)
				}
			}
		}
	}()

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}
	errc := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.Addr, "secure_cookies", cfg.SecureCookies)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdown); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	return nil
}

// ensurePrincipal creates the principal account on first run. By default nobody knows a
// password: a one-time setup link is printed instead, and the principal chooses their own.
// SCHOOL_BOOTSTRAP_PASSWORD can be used instead for automated installs.
func ensurePrincipal(ctx context.Context, store *repository.Store, cfg *config.Config, log *slog.Logger) error {
	has, err := store.HasPrincipal(ctx)
	if err != nil || has {
		return err
	}
	if cfg.BootstrapPassword != "" {
		if err := security.ValidatePassword(cfg.BootstrapPassword, cfg.BootstrapUser); err != nil {
			return errors.New("SCHOOL_BOOTSTRAP_PASSWORD is too weak: " + err.Error())
		}
		hash, err := security.HashPassword(cfg.BootstrapPassword)
		if err != nil {
			return err
		}
		if _, err := store.CreatePrincipal(ctx, cfg.BootstrapUser, repository.NewUser{FullName: cfg.BootstrapName, PasswordHash: hash}); err != nil {
			return err
		}
		log.Info("principal account created", "username", cfg.BootstrapUser)
		return nil
	}

	hash, err := security.UnusablePassword()
	if err != nil {
		return err
	}
	id, err := store.CreatePrincipal(ctx, cfg.BootstrapUser, repository.NewUser{FullName: cfg.BootstrapName, PasswordHash: hash})
	if err != nil {
		return err
	}
	token, tokenHash, err := security.NewToken()
	if err != nil {
		return err
	}
	if err := store.CreateActivation(ctx, id, tokenHash, time.Now().Add(72*time.Hour)); err != nil {
		return err
	}
	base := cfg.BaseURL
	if base == "" {
		host := cfg.Addr
		if strings.HasPrefix(host, ":") {
			host = "localhost" + host
		}
		base = "http://" + host
	}
	log.Warn("principal account created. Open this link within 72 hours to choose a password",
		"username", cfg.BootstrapUser, "setup_link", base+"/activate/"+token)
	return nil
}

// printSetupLink is the recovery tool for whoever runs the server: it makes the account
// "awaiting setup" (the old password stops working, sessions end) and prints a new 72-hour link.
func printSetupLink(ctx context.Context, store *repository.Store, cfg *config.Config, username string) error {
	rec, err := store.GetUserForLogin(ctx, strings.ToLower(strings.TrimSpace(username)))
	if err != nil {
		return fmt.Errorf("no account with username %q", username)
	}
	hash, err := security.UnusablePassword()
	if err != nil {
		return err
	}
	if err := store.MarkPendingSetup(ctx, rec.User.ID, hash); err != nil {
		return err
	}
	token, tokenHash, err := security.NewToken()
	if err != nil {
		return err
	}
	if err := store.CreateActivation(ctx, rec.User.ID, tokenHash, time.Now().Add(72*time.Hour)); err != nil {
		return err
	}
	base := cfg.BaseURL
	if base == "" {
		host := cfg.Addr
		if strings.HasPrefix(host, ":") {
			host = "localhost" + host
		}
		base = "http://" + host
	}
	fmt.Printf("\nSetup link for %s (%s), valid for 72 hours:\n\n  %s/activate/%s\n\n", rec.User.FullName, rec.User.Username, base, token)
	return nil
}
