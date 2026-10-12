package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

type Middleware func(http.Handler) http.Handler

// Chain wraps h so the first middleware listed is the outermost.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func Recover(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if v := recover(); v != nil {
					log.Error("panic", "path", r.URL.Path, "error", v)
					http.Error(w, "Something went wrong.", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func Logger(log *slog.Logger, trustProxy bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			log.Info("request", "method", r.Method, "path", r.URL.Path, "status", sw.status,
				"ms", time.Since(start).Milliseconds(), "ip", ClientIP(r, trustProxy))
		})
	}
}

// LimitBody caps request bodies at 1 MiB.
func LimitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		next.ServeHTTP(w, r)
	})
}
