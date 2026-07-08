// Package logger is the Tier-1 platform observability seam: structured slog JSON
// logging with base correlation fields (trace_id/span_id/correlation_id/actor/module)
// plus W3C traceparent carriage. Reclassified from Tier-0 kernel/obs to Tier-1
// platform by WS-E-d (Task 8) per AC-E4.
package logger

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"strings"
)

type ctxKey int

const (
	keyTrace ctxKey = iota
	keySpan
	keyCorrelation
	keyActor
	keyModule
)

// NewJSONHandler returns a slog JSON handler writing to w.
func NewJSONHandler(w io.Writer) slog.Handler {
	return slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo})
}

// WithTrace attaches a trace id and span id to ctx for structured logging.
func WithTrace(ctx context.Context, traceID, spanID string) context.Context {
	ctx = context.WithValue(ctx, keyTrace, traceID)
	return context.WithValue(ctx, keySpan, spanID)
}

// WithCorrelation attaches a correlation id to ctx for structured logging.
func WithCorrelation(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyCorrelation, id)
}

// WithActor attaches the acting principal to ctx for structured logging.
func WithActor(ctx context.Context, actor string) context.Context {
	return context.WithValue(ctx, keyActor, actor)
}

// WithModule attaches the originating module to ctx for structured logging.
func WithModule(ctx context.Context, module string) context.Context {
	return context.WithValue(ctx, keyModule, module)
}

func str(ctx context.Context, k ctxKey) string {
	if v, ok := ctx.Value(k).(string); ok {
		return v
	}
	return ""
}

// TraceID returns the ctx trace id (empty if unset).
func TraceID(ctx context.Context) string { return str(ctx, keyTrace) }

// FormatTraceparent builds a W3C traceparent (version 00, sampled).
func FormatTraceparent(traceID, spanID string) string {
	return "00-" + traceID + "-" + spanID + "-01"
}

// ParseTraceparent parses a W3C traceparent header.
func ParseTraceparent(h string) (traceID, spanID string, ok bool) {
	parts := strings.Split(h, "-")
	if len(parts) != 4 || parts[0] != "00" || len(parts[1]) != 32 || len(parts[2]) != 16 {
		return "", "", false
	}
	return parts[1], parts[2], true
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// NewTraceID returns a fresh 32-hex-char W3C trace id.
func NewTraceID() string { return randHex(16) }

// NewSpanID returns a fresh 16-hex-char W3C span id.
func NewSpanID() string { return randHex(8) }

// Logger returns the default slog logger enriched with the ctx base fields.
func Logger(ctx context.Context) *slog.Logger {
	attrs := []any{}
	add := func(field string, k ctxKey) {
		if v := str(ctx, k); v != "" {
			attrs = append(attrs, slog.String(field, v))
		}
	}
	add("trace_id", keyTrace)
	add("span_id", keySpan)
	add("correlation_id", keyCorrelation)
	add("actor", keyActor)
	add("module", keyModule)
	return slog.Default().With(attrs...)
}
