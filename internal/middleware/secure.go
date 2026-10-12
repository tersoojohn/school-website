package middleware

import (
	"net/http"
	"strings"
)

// SecureHeaders sets browser protections on every response. The CSP only allows scripts and
// styles served from this site (no inline code), images from this site and the configured
// image hosts, and forbids framing and cross-site form posts.
func SecureHeaders(secure bool, imageHosts []string) Middleware {
	img := "'self'"
	for _, h := range imageHosts {
		img += " " + h
	}
	csp := "default-src 'none'; script-src 'self'; style-src 'self'; img-src " + img + "; font-src 'self'; " +
		"form-action 'self'; frame-ancestors 'none'; base-uri 'none'"
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("Content-Security-Policy", csp)
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "same-origin")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			if secure {
				h.Set("Strict-Transport-Security", "max-age=31536000")
			}
			if !strings.HasPrefix(r.URL.Path, "/static/") {
				// Pages contain personal data: never let browsers or proxies keep a copy.
				h.Set("Cache-Control", "no-store")
			}
			next.ServeHTTP(w, r)
		})
	}
}
