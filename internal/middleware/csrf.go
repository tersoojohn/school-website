package middleware

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

// OriginPolicy decides which Origin/Referer hosts count as "this site". The request's own Host
// always counts. Behind a reverse proxy the Host header is often rewritten, so the public
// address from SCHOOL_BASE_URL (and X-Forwarded-Host when the proxy is trusted) counts too.
type OriginPolicy struct {
	Hosts      []string
	TrustProxy bool
	Log        *slog.Logger
}

// NewOriginPolicy builds the policy from the configured public base URL.
func NewOriginPolicy(baseURL string, trustProxy bool, log *slog.Logger) *OriginPolicy {
	p := &OriginPolicy{TrustProxy: trustProxy, Log: log}
	if u, err := url.Parse(baseURL); err == nil && u.Host != "" {
		p.Hosts = append(p.Hosts, u.Host)
	}
	return p
}

// Same reports whether a browser-sent Origin or Referer header belongs to this site.
func (p *OriginPolicy) Same(r *http.Request) bool {
	src := r.Header.Get("Origin")
	if src == "" || src == "null" {
		src = r.Header.Get("Referer")
	}
	if src == "" {
		return true // not a browser form post; the token check still applies
	}
	u, err := url.Parse(src)
	if err != nil {
		return false
	}
	if u.Host == r.Host {
		return true
	}
	for _, h := range p.Hosts {
		if strings.EqualFold(u.Host, h) {
			return true
		}
	}
	if p.TrustProxy {
		if fh := r.Header.Get("X-Forwarded-Host"); fh != "" && strings.EqualFold(u.Host, strings.TrimSpace(strings.Split(fh, ",")[0])) {
			return true
		}
	}
	if p.Log != nil {
		p.Log.Warn("request blocked: Origin does not match this site",
			"origin", src, "host_header", r.Host, "fix", "set SCHOOL_BASE_URL to the address people use, and make your proxy pass the Host header")
	}
	return false
}

// CSRF protects every state-changing request from a signed-in user. Two independent checks:
// the Origin/Referer header must match this site, and the form must carry the session's token.
func CSRF(deny http.HandlerFunc, origin *OriginPolicy) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}
			sess := SessionFrom(r.Context())
			if sess == nil || !origin.Same(r) {
				deny(w, r)
				return
			}
			got := r.PostFormValue("csrf_token")
			if subtle.ConstantTimeCompare([]byte(got), []byte(sess.CSRF)) != 1 {
				deny(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
