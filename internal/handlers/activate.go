package handlers

import (
	"errors"
	"net/http"
	"schoolportal/internal/middleware"

	"schoolportal/internal/repository"
	"schoolportal/internal/security"
)

type activateView struct {
	Invalid  bool
	Name     string
	Username string
	Role     string
	Error    string
}

func (a *App) ActivateForm(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer") // the link is a secret; never leak it
	user, err := a.Store.GetActivation(r.Context(), security.HashToken(r.PathValue("token")))
	if errors.Is(err, repository.ErrNotFound) {
		a.render(w, r, http.StatusNotFound, "auth/activate", "Setup link", activateView{Invalid: true})
		return
	}
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, "auth/activate", "Set your password",
		activateView{Name: user.FullName, Username: user.Username, Role: user.Role.Label()})
}

func (a *App) ActivateSubmit(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	if !a.Origin.Same(r) {
		a.Forbidden(w, r)
		return
	}
	hash := security.HashToken(r.PathValue("token"))
	user, err := a.Store.GetActivation(r.Context(), hash)
	if errors.Is(err, repository.ErrNotFound) {
		a.render(w, r, http.StatusNotFound, "auth/activate", "Setup link", activateView{Invalid: true})
		return
	}
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	view := activateView{Name: user.FullName, Username: user.Username, Role: user.Role.Label()}
	show := func(msg string) {
		view.Error = msg
		a.render(w, r, http.StatusUnprocessableEntity, "auth/activate", "Set your password", view)
	}

	password, confirm := r.PostFormValue("new"), r.PostFormValue("confirm")
	if password != confirm {
		show("The two passwords do not match.")
		return
	}
	if err := security.ValidatePassword(password, user.Username); err != nil {
		show(err.Error())
		return
	}
	pwHash, err := security.HashPassword(password)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	user, err = a.Store.CompleteActivation(r.Context(), hash, pwHash)
	if errors.Is(err, repository.ErrNotFound) { // used or expired between the two requests
		a.render(w, r, http.StatusNotFound, "auth/activate", "Setup link", activateView{Invalid: true})
		return
	}
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	a.dropCurrentSession(r)
	if err := a.Store.RecordLogin(r.Context(), user.ID); err != nil {
		a.ServerError(w, r, err)
		return
	}
	if err := a.startSession(w, r, user.ID); err != nil {
		a.ServerError(w, r, err)
		return
	}
	a.audit(r, user.ID, "account_activated", "")
	a.redirect(w, r, user.Role.Home(), "ok", "Your password is set. Welcome, "+user.FullName+".")
}

// dropCurrentSession ends whatever session this browser already holds.
func (a *App) dropCurrentSession(r *http.Request) {
	if old, err := r.Cookie(middleware.SessionCookie(a.Cfg.SecureCookies)); err == nil {
		_ = a.Store.DeleteSession(r.Context(), security.HashToken(old.Value))
	}
}
