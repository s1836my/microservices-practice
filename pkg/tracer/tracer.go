package tracer

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Config は OpenTelemetry TracerProvider の設定。
type Config struct {
	ServiceName    string
	ServiceVersion string
	// Endpoint は OTLP HTTP エンドポイント URL。例: "http://jaeger:4318"
	// 空文字のとき noop プロバイダーを使う（テスト・開発向け）。
	Endpoint   string
	SampleRate float64 // 0.0(never) ～ 1.0(always)。デフォルト 1.0
}

// New は TracerProvider を初期化し、otel のグローバルプロバイダーに設定する。
// 返された shutdown 関数を defer で呼ぶこと。
func New(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create otel resource: %w", err)
	}

	sampler := buildSampler(cfg.SampleRate)

	var tp *sdktrace.TracerProvider
	if cfg.Endpoint == "" {
		// noop: エクスポーターなし (テスト・開発向け)
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sampler),
		)
	} else {
		exp, err := otlptracehttp.New(ctx,
			otlptracehttp.WithEndpointURL(cfg.Endpoint),
			otlptracehttp.WithInsecure(),
		)
		if err != nil {
			return nil, fmt.Errorf("create OTLP exporter: %w", err)
		}
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exp),
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sampler),
		)
	}

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// SpanFromContext は context から現在の Span を返すヘルパー。
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

func buildSampler(rate float64) sdktrace.Sampler {
	switch {
	case rate <= 0:
		return sdktrace.NeverSample()
	case rate >= 1:
		return sdktrace.AlwaysSample()
	default:
		return sdktrace.TraceIDRatioBased(rate)
	}
}
