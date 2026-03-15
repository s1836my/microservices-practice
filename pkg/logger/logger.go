package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Config はロガーの初期化設定。
type Config struct {
	Level       string    // "debug" | "info" | "warn" | "error"
	Format      string    // "json" (default) | "text"
	ServiceName string
	Output      io.Writer // nil のとき os.Stdout を使用
}

// New は Config から *slog.Logger を生成し、slog のデフォルトロガーにもセットして返す。
func New(cfg Config) *slog.Logger {
	out := cfg.Output
	if out == nil {
		out = os.Stdout
	}

	opts := &slog.HandlerOptions{Level: ParseLevel(cfg.Level)}
	var handler slog.Handler
	if strings.EqualFold(cfg.Format, "text") {
		handler = slog.NewTextHandler(out, opts)
	} else {
		handler = slog.NewJSONHandler(out, opts)
	}

	l := slog.New(handler)
	if cfg.ServiceName != "" {
		l = l.With("service", cfg.ServiceName)
	}

	slog.SetDefault(l)
	return l
}

// ParseLevel は文字列を slog.Level に変換する。未知の値は LevelInfo を返す。
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
