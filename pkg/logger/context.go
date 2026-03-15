package logger

import (
	"context"
	"log/slog"
)

type contextKey struct{}

// WithLogger は context にロガーを埋め込む。
func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

// FromContext は context からロガーを取り出す。見つからなければ slog.Default() を返す。
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(contextKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}
