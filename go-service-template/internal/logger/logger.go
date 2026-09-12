package logger

import (
	"context"
	"os"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace"
)

func New(serviceName, environment, level string) zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339Nano

	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	return zerolog.New(os.Stdout).
		Level(lvl).
		With().
		Timestamp().
		Str("service", serviceName).
		Str("env", environment).
		Caller().
		Logger()
}

type ctxKey struct{}

// WithLogger stores a logger on the context (e.g. after adding request-scoped
// fields), so downstream code can retrieve it via FromContext.
func WithLogger(ctx context.Context, l zerolog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext returns the request-scoped logger with trace_id/span_id
// attached from any active OTel span found on the context. Falls back to a
// bare stdout logger if nothing was stored (shouldn't happen in handler code
// wired through the otelhttp/otelgrpc middleware).
func FromContext(ctx context.Context) zerolog.Logger {
	l, ok := ctx.Value(ctxKey{}).(zerolog.Logger)
	if !ok {
		l = zerolog.New(os.Stdout).With().Timestamp().Logger()
	}

	span := trace.SpanContextFromContext(ctx)
	if span.IsValid() {
		l = l.With().
			Str("trace_id", span.TraceID().String()).
			Str("span_id", span.SpanID().String()).
			Logger()
	}
	return l
}
