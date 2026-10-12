// Package handlers contains the HTTP handlers, grouped by who uses them.
package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"schoolportal/internal/config"
	"schoolportal/internal/middleware"
	"schoolportal/internal/repository"
)

type App struct {
	Cfg   *config.Config
	Store *repository.Store
	View  *Renderer
	Log   *slog.Logger
	// Origin decides which Origin/Referer headers count as this site.
	Origin *middleware.OriginPolicy
}

func New(cfg *config.Config, store *repository.Store, log *slog.Logger) (*App, error) {
	view, err := NewRenderer(cfg.SchoolName)
	if err != nil {
		return nil, err
	}
	return &App{Cfg: cfg, Store: store, View: view, Log: log,
		Origin: middleware.NewOriginPolicy(cfg.BaseURL, cfg.TrustProxy, log)}, nil
}

func (a *App) ip(r *http.Request) string { return middleware.ClientIP(r, a.Cfg.TrustProxy) }

// audit records a security-relevant event. Failures are logged, never shown to the user.
func (a *App) audit(r *http.Request, userID int64, action, detail string) {
	if err := a.Store.Audit(r.Context(), userID, action, detail, a.ip(r)); err != nil {
		a.Log.Error("audit write failed", "action", action, "error", err)
	}
}

func (a *App) Forbidden(w http.ResponseWriter, r *http.Request) {
	a.errorPage(w, r, http.StatusForbidden, "You do not have access to this page.")
}

func (a *App) NotFound(w http.ResponseWriter, r *http.Request) {
	a.errorPage(w, r, http.StatusNotFound, "We could not find that page.")
}

func (a *App) ServerError(w http.ResponseWriter, r *http.Request, err error) {
	a.Log.Error("server error", "path", r.URL.Path, "error", err)
	a.errorPage(w, r, http.StatusInternalServerError, "Something went wrong on our side. Try again in a moment.")
}

func (a *App) BadRequest(w http.ResponseWriter, r *http.Request, msg string) {
	a.errorPage(w, r, http.StatusBadRequest, msg)
}

func (a *App) errorPage(w http.ResponseWriter, r *http.Request, status int, msg string) {
	a.render(w, r, status, "error", http.StatusText(status), map[string]any{"Message": msg, "Status": status})
}

func (a *App) redirect(w http.ResponseWriter, r *http.Request, to string, kind, msg string) {
	if msg != "" {
		a.setFlash(w, kind, msg)
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
}

func pathID(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id, err == nil && id > 0
}

func formID(r *http.Request, name string) int64 {
	id, _ := strconv.ParseInt(r.PostFormValue(name), 10, 64)
	return id
}
