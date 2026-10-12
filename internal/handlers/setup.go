package handlers

import (
	"net/http"
	"time"

	"schoolportal/internal/models"
	"schoolportal/internal/security"
)

// setupTTL is how long a one-time setup link stays valid.
const setupTTL = 72 * time.Hour

type setupView struct {
	Name      string
	Username  string
	Role      string
	Link      string
	Expires   time.Time
	BackPath  string
	BackLabel string
}

// BaseURL is the public address of the site, used to build setup links.
func (a *App) baseURL(r *http.Request) string {
	if a.Cfg.BaseURL != "" {
		return a.Cfg.BaseURL
	}
	scheme := "http"
	if r.TLS != nil || (a.Cfg.TrustProxy && r.Header.Get("X-Forwarded-Proto") == "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// newSetupLink creates a single-use link that lets the account owner choose their own password.
// Nobody, including the person issuing the link, ever sees or knows a password.
func (a *App) newSetupLink(r *http.Request, userID int64) (string, time.Time, error) {
	token, hash, err := security.NewToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expires := time.Now().Add(setupTTL)
	if err := a.Store.CreateActivation(r.Context(), userID, hash, expires); err != nil {
		return "", time.Time{}, err
	}
	return a.baseURL(r) + "/activate/" + token, expires, nil
}

// showSetupLink renders the page an administrator uses to hand over the link.
func (a *App) showSetupLink(w http.ResponseWriter, r *http.Request, userID int64, name, username string, role models.Role, back, backLabel string) {
	link, expires, err := a.newSetupLink(r, userID)
	if err != nil {
		a.ServerError(w, r, err)
		return
	}
	a.render(w, r, http.StatusOK, "manage/setup_link", "Setup link", setupView{
		Name: name, Username: username, Role: role.Label(), Link: link, Expires: expires, BackPath: back, BackLabel: backLabel})
}
