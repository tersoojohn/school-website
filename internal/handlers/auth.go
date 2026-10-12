package handlers

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"schoolportal/internal/middleware"
	"schoolportal/internal/models"
	"schoolportal/internal/repository"
	"schoolportal/internal/security"
)

const (
	maxFailedLogins = 5
	lockDuration    = 15 * time.Minute
	loginCookie     = "login_csrf"
)

type loginView struct {
	Token    string
	Username string
	Error    string
}

func (a *App) LoginForm(w http.ResponseWriter, r *http.Request) {
	if middleware.UserFrom(r.Context()) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	a.showLogin(w, r, http.StatusOK, "", "")
}

func (a *App) showLogin(w http.ResponseWriter, r *http.Request, status int, username, msg string) {
	token, _, err := security.NewToken()
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	// The sign-in form has no session yet, so it uses a one-off cookie/field pair
	// to stop other sites from submitting a login on the visitor's behalf.
	http.SetCookie(w, &http.Cookie{Name: loginCookie, Value: token, Path: "/login", MaxAge: 900,
		HttpOnly: true, Secure: a.Cfg.SecureCookies, SameSite: http.SameSiteStrictMode})
	a.render(w, r, status, "auth/login", "Sign in", loginView{Token: token, Username: username, Error: msg})
}

func (a *App) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		a.BadRequest(w, r, "That form could not be read.")
		return
	}
	// Sign-in has no session yet, so it is protected two ways. The form token must match the
	// one-off cookie, OR the browser's Origin/Referer must prove the form came from this site.
	// Either is enough, so a dropped cookie or a second open tab does not lock people out,
	// while a form posted from another website is still refused.
	c, cerr := r.Cookie(loginCookie)
	cookieOK := cerr == nil && subtle.ConstantTimeCompare([]byte(c.Value), []byte(r.PostFormValue("csrf_token"))) == 1
	if !cookieOK {
		hasSource := r.Header.Get("Origin") != "" || r.Header.Get("Referer") != ""
		if !hasSource || !a.Origin.Same(r) {
			a.Log.Warn("sign-in form rejected", "cookie_present", cerr == nil, "has_origin_or_referer", hasSource,
				"origin", r.Header.Get("Origin"), "host_header", r.Host)
			a.showLogin(w, r, http.StatusBadRequest, "", "The sign-in page expired. Please try again.")
			return
		}
	}

	username := strings.ToLower(strings.TrimSpace(r.PostFormValue("username")))
	password := r.PostFormValue("password")
	const generic = "Wrong username or password, or the account is locked for a few minutes."
	fail := func() { a.showLogin(w, r, http.StatusUnauthorized, username, generic) }

	if username == "" || len(username) > 64 || password == "" || len(password) > 256 {
		fail()
		return
	}

	rec, err := a.Store.GetUserForLogin(r.Context(), username)
	if errors.Is(err, repository.ErrNotFound) {
		security.BurnTime(password)
		a.audit(r, 0, "login_failed", "unknown username")
		fail()
		return
	}
	if err != nil {
		a.ServerError(w, r, err)
		return
	}

	if time.Now().Before(rec.LockedUntil) {
		a.audit(r, rec.User.ID, "login_blocked", "account locked")
		fail()
		return
	}
	if !security.VerifyPassword(password, rec.Hash) {
		if err := a.Store.RecordFailedLogin(r.Context(), rec.User.ID, maxFailedLogins, lockDuration); err != nil {
			a.Log.Error("record failed login", "error", err)
		}
		a.audit(r, rec.User.ID, "login_failed", "wrong password")
		fail()
		return
	}
	if !rec.User.Active {
		a.audit(r, rec.User.ID, "login_blocked", "account deactivated")
		fail()
		return
	}

	// Drop any session this browser already had, then issue a fresh one (prevents session fixation).
	a.dropCurrentSession(r)
	if err := a.Store.RecordLogin(r.Context(), rec.User.ID); err != nil {
		a.ServerError(w, r, err)
		return
	}
	if err := a.startSession(w, r, rec.User.ID); err != nil {
		a.ServerError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: loginCookie, Value: "", Path: "/login", MaxAge: -1})
	a.audit(r, rec.User.ID, "login", "")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) startSession(w http.ResponseWriter, r *http.Request, userID int64) error {
	token, hash, err := security.NewToken()
	if err != nil {
		return err
	}
	csrf, _, err := security.NewToken()
	if err != nil {
		return err
	}
	now := time.Now()
	sess := models.Session{TokenHash: hash, UserID: userID, CSRF: csrf, LastSeen: now, ExpiresAt: now.Add(a.Cfg.SessionMax)}
	if err := a.Store.CreateSession(r.Context(), sess, a.ip(r), r.UserAgent()); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: middleware.SessionCookie(a.Cfg.SecureCookies), Value: token, Path: "/",
		MaxAge: int(a.Cfg.SessionMax.Seconds()), HttpOnly: true, Secure: a.Cfg.SecureCookies, SameSite: http.SameSiteLaxMode})
	return nil
}

func (a *App) Logout(w http.ResponseWriter, r *http.Request) {
	if s := middleware.SessionFrom(r.Context()); s != nil {
		_ = a.Store.DeleteSession(r.Context(), s.TokenHash)
		a.audit(r, s.UserID, "logout", "")
	}
	http.SetCookie(w, &http.Cookie{Name: middleware.SessionCookie(a.Cfg.SecureCookies), Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: a.Cfg.SecureCookies, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

type passwordView struct {
	Forced bool
	Error  string
}

func (a *App) PasswordForm(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	a.render(w, r, http.StatusOK, "auth/password", "Change password", passwordView{Forced: u.MustChangePassword})
}

func (a *App) PasswordSubmit(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	show := func(msg string) {
		a.render(w, r, http.StatusUnprocessableEntity, "auth/password", "Change password", passwordView{Forced: u.MustChangePassword, Error: msg})
	}
	current, next, confirm := r.PostFormValue("current"), r.PostFormValue("new"), r.PostFormValue("confirm")

	hash, err := a.Store.GetPasswordHash(r.Context(), u.ID)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	if len(current) > 256 || !security.VerifyPassword(current, hash) {
		a.audit(r, u.ID, "password_change_failed", "wrong current password")
		show("Your current password is not correct.")
		return
	}
	if next != confirm {
		show("The new password and its confirmation do not match.")
		return
	}
	if next == current {
		show("Choose a password different from the current one.")
		return
	}
	if err := security.ValidatePassword(next, u.Username); err != nil {
		show(err.Error())
		return
	}
	newHash, err := security.HashPassword(next)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	if err := a.Store.UpdatePassword(r.Context(), u.ID, newHash, false); err != nil {
		a.ServerError(w, r, err)
		return
	}
	// Sign out every device, then sign this one back in with a fresh session.
	_ = a.Store.DeleteUserSessions(r.Context(), u.ID)
	if err := a.startSession(w, r, u.ID); err != nil {
		a.ServerError(w, r, err)
		return
	}
	a.audit(r, u.ID, "password_changed", "")
	a.redirect(w, r, u.Role.Home(), "ok", "Password changed. Other devices have been signed out.")
}

// Home sends each signed-in user to their own dashboard.
func (a *App) Home(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, middleware.UserFrom(r.Context()).Role.Home(), http.StatusSeeOther)
}
