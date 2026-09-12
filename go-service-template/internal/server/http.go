// Package server wires up the HTTP and gRPC servers. HTTP gets OTel tracing
// middleware + a logging middleware that injects a request-scoped, trace-id
// correlated logger into the context. pprof is mounted on its own listener
// (PprofPort) so it's never accidentally exposed on the public HTTP port.
package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/joserafaelSH/golang-software-engineer/go-service-template/internal/handlers"
	"github.com/joserafaelSH/golang-software-engineer/go-service-template/internal/logger"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// NewHTTPServer builds the app-facing HTTP server: business routes wrapped
// in OTel tracing + structured request logging.
func NewHTTPServer(port int, baseLogger zerolog.Logger) *http.Server {
	r := chi.NewRouter()
	r.Use(chimiddleware.Recoverer) // recover from panics in handlers -> 500 instead of crashing the process
	r.Use(loggingMiddleware(baseLogger))

	r.Get("/healthz", handlers.Health)
	r.Get("/readyz", handlers.Ready)
	// Add real routes here, e.g.:
	// r.Route("/v1", func(r chi.Router) {
	//     r.Get("/widgets/{id}", handlers.GetWidget)
	// })
	// Every handler downstream of loggingMiddleware can call
	// logger.FromContext(r.Context()) for trace-id-correlated logs.

	instrumented := otelhttp.NewHandler(r, "http.server")

	return &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      instrumented,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// NewPprofServer exposes net/http/pprof on a dedicated port. In cluster,
// keep this off the Service that gets external traffic — the Helm chart
// only ClusterIP-exposes it, and Pyroscope's Alloy agent scrapes it directly.
func NewPprofServer(port int) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	return &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
}

func loggingMiddleware(base zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ctx := logger.WithLogger(r.Context(), base)
			r = r.WithContext(ctx)

			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)

			l := logger.FromContext(r.Context())
			l.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", sw.status).
				Dur("duration_ms", time.Since(start)).
				Msg("request handled")
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// Shutdown gracefully drains a server; call from a deferred/signal-handling
// path in main() with a bounded context.
func Shutdown(ctx context.Context, srv *http.Server) error {
	return srv.Shutdown(ctx)
}
