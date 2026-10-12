package middleware

import (
	"context"
	"net/http"
	"time"

	"schoolportal/internal/config"
	"schoolportal/internal/models"
	"schoolportal/internal/repository"
	"schoolportal/internal/security"
)

// Authenticate looks up the session cookie and, when valid, puts the user and session
// in the request context. It never rejects a request by itself; see RequireLogin.
func Authenticate(store *repository.Store, cfg *config.Config) Middleware {
	name := SessionCookie(cfg.SecureCookies)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if c, err := r.Cookie(name); err == nil && c.Value != "" && len(c.Value) <= 128 {
				hash := security.HashToken(c.Value)
				sess, user, err := store.GetSession(ctx, hash)
				now := time.Now()
				switch {
				case err == nil && user.Active && now.Before(sess.ExpiresAt) && now.Sub(sess.LastSeen) < cfg.SessionIdle:
					if now.Sub(sess.LastSeen) > time.Minute {
						_ = store.TouchSession(ctx, hash, now)
					}
					ctx = context.WithValue(ctx, userKey, user)
					ctx = context.WithValue(ctx, sessionKey, sess)
				default:
					if err == nil {
						_ = store.DeleteSession(ctx, hash)
					}
					http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1,
						HttpOnly: true, Secure: cfg.SecureCookies, SameSite: http.SameSiteLaxMode})
				}
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireLogin sends anonymous visitors to the sign-in page, and sends anyone still on
// a temporary password to the change-password page before they can reach anything else.
func RequireLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFrom(r.Context())
		if u == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if u.MustChangePassword && r.URL.Path != "/account/password" && r.URL.Path != "/logout" {
			http.Redirect(w, r, "/account/password", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireRole allows only the listed roles. deny renders the 403 page.
func RequireRole(deny http.HandlerFunc, roles ...models.Role) Middleware {
	allowed := make(map[models.Role]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := UserFrom(r.Context())
			if u == nil || !allowed[u.Role] {
				deny(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
