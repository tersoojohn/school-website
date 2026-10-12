// Package middleware contains the HTTP layers that wrap every request:
// recovery, logging, security headers, sessions, role checks, CSRF and rate limiting.
package middleware

import (
	"context"

	"schoolportal/internal/models"
)

type ctxKey int

const (
	userKey ctxKey = iota
	sessionKey
)

func UserFrom(ctx context.Context) *models.User {
	u, _ := ctx.Value(userKey).(*models.User)
	return u
}

func SessionFrom(ctx context.Context) *models.Session {
	s, _ := ctx.Value(sessionKey).(*models.Session)
	return s
}

// SessionCookie returns the cookie name. The __Host- prefix makes browsers reject the
// cookie unless it is Secure, host-only and scoped to "/", so it can only be used over HTTPS.
func SessionCookie(secure bool) string {
	if secure {
		return "__Host-sid"
	}
	return "sid"
}
