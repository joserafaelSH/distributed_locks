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
)

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
