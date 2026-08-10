package logger

import (
	"context"
	"log/slog"
	"os"
)

type ctxKey string

const TraceIDKey ctxKey = "trace_id"

func New(env string) *slog.Logger {
	ctxHandler := &ContextHandler{
		Handler: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}),
	}

	if env == "local" {
		ctxHandler.Handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	}

	return slog.New(ctxHandler)
}

func AppendCtx(ctx context.Context, key ctxKey, value string) context.Context {
	return context.WithValue(ctx, key, value)
}

type ContextHandler struct {
	slog.Handler
}

func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok {
		r.AddAttrs(slog.String("trace_id", traceID))
	}

	return h.Handler.Handle(ctx, r)
}
