package tracer_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"

	"github.com/yourorg/micromart/pkg/tracer"
)

func TestNew_NoEndpoint(t *testing.T) {
	ctx := context.Background()

	shutdown, err := tracer.New(ctx, tracer.Config{
		ServiceName: "test-service",
		SampleRate:  1.0,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if shutdown == nil {
		t.Fatal("shutdown function should not be nil")
	}

	// グローバルプロバイダーが設定されていること
	tp := otel.GetTracerProvider()
	if tp == nil {
		t.Error("TracerProvider should be set globally")
	}

	if err := shutdown(ctx); err != nil {
		t.Errorf("shutdown() error = %v", err)
	}
}

func TestNew_SampleRates(t *testing.T) {
	cases := []struct {
		rate float64
		name string
	}{
		{0.0, "never"},
		{0.5, "ratio"},
		{1.0, "always"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			shutdown, err := tracer.New(ctx, tracer.Config{
				ServiceName: "svc",
				SampleRate:  tc.rate,
			})
			if err != nil {
				t.Fatalf("New(rate=%f) error = %v", tc.rate, err)
			}
			defer shutdown(ctx) //nolint:errcheck
		})
	}
}

func TestNew_WithEndpoint(t *testing.T) {
	ctx := context.Background()

	// OTLP エクスポーターの生成はエンドポイントへの接続をしない（lazy connect）ので成功する
	shutdown, err := tracer.New(ctx, tracer.Config{
		ServiceName:    "test-service",
		ServiceVersion: "v1.0.0",
		Endpoint:       "http://localhost:4318",
		SampleRate:     0.5,
	})
	if err != nil {
		t.Fatalf("New() with endpoint error = %v", err)
	}
	if shutdown == nil {
		t.Fatal("shutdown function should not be nil")
	}
	// shutdown はエクスポーターへのフラッシュを試みるが、接続不能なのでエラーになる場合がある
	// エラーの有無は問わない（接続不可能な環境でのテストのため）
	_ = shutdown(ctx)
}

func TestSpanFromContext(t *testing.T) {
	ctx := context.Background()
	span := tracer.SpanFromContext(ctx)
	if span == nil {
		t.Error("SpanFromContext should return a span (noop) even without active span")
	}
}
