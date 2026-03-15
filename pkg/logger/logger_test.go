package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/yourorg/micromart/pkg/logger"
)

func TestNew_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	l := logger.New(logger.Config{
		Level:       "debug",
		Format:      "json",
		ServiceName: "test-svc",
		Output:      &buf,
	})

	l.Info("hello world", "key", "value")

	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("failed to parse JSON log: %v\noutput: %s", err, buf.String())
	}

	if got := out["msg"]; got != "hello world" {
		t.Errorf("msg = %q, want %q", got, "hello world")
	}
	if got := out["service"]; got != "test-svc" {
		t.Errorf("service = %q, want %q", got, "test-svc")
	}
	if got := out["key"]; got != "value" {
		t.Errorf("key = %q, want %q", got, "value")
	}
}

func TestNew_TextFormat(t *testing.T) {
	var buf bytes.Buffer
	l := logger.New(logger.Config{
		Format: "text",
		Output: &buf,
	})
	l.Info("text message")
	if buf.Len() == 0 {
		t.Error("expected non-empty output")
	}
}

func TestNew_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	l := logger.New(logger.Config{
		Level:  "warn",
		Output: &buf,
	})

	l.Debug("should be filtered")
	l.Info("should be filtered")

	if buf.Len() != 0 {
		t.Errorf("expected empty output for debug/info below warn level, got: %s", buf.String())
	}

	l.Warn("should appear")
	if buf.Len() == 0 {
		t.Error("expected warn message to appear")
	}
}

func TestParseLevel(t *testing.T) {
	cases := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"unknown", slog.LevelInfo},
		{"", slog.LevelInfo},
	}
	for _, tc := range cases {
		got := logger.ParseLevel(tc.input)
		if got != tc.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestWithLogger_FromContext(t *testing.T) {
	var buf bytes.Buffer
	l := logger.New(logger.Config{Output: &buf})

	ctx := context.Background()
	ctx = logger.WithLogger(ctx, l.With("request_id", "abc123"))

	got := logger.FromContext(ctx)
	got.Info("test")

	if buf.Len() == 0 {
		t.Error("expected output from context logger")
	}
}

func TestFromContext_Default(t *testing.T) {
	ctx := context.Background()
	l := logger.FromContext(ctx)
	if l == nil {
		t.Error("FromContext should return default logger, got nil")
	}
}
