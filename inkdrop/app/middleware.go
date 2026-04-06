package app

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func RegisterMiddleWares(r *chi.Mux) {
	// default Chi logger prints status and response size (often 0 with redirects)
	// we add a simple middleware that also reports incoming body size for POSTs
	r.Use(RequestBodyLogger)
	r.Use(StatsMiddleware)
	r.Use(SecureHeaders)
	r.Use(middleware.Logger)
	r.Use(middleware.StripSlashes)
}

// responseRecorder wraps http.ResponseWriter to capture status and bytes written
type responseRecorder struct {
	http.ResponseWriter
	status  int
	written int64
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.written += int64(n)
	return n, err
}

// StatsMiddleware logs request method/path, response status, bytes written and elapsed time.
func StatsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rr := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rr, r)
		elapsed := time.Since(start)
		// Simple log that can be grepped by automation
		println("[stats]", r.Method, r.URL.Path, "status", rr.status, "bytes", rr.written, "elapsed", elapsed.String())
	})
}

func SecureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; media-src 'self' blob:; connect-src 'self' https://cdn.jsdelivr.net blob:; object-src 'none'; frame-ancestors 'none'; base-uri 'self';")
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}
		next.ServeHTTP(w, r)
	})
}

// RequestBodyLogger logs the ContentLength of incoming requests with bodies.
func RequestBodyLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > 0 {
			// content length can be 0 when client doesn't send a body
			// this helps diagnose why login seems to send "no data"
			println("[req]", r.Method, r.URL.String(), "body len", r.ContentLength)
		}
		next.ServeHTTP(w, r)
	})
}
