package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yourorg/micromart/pkg/logger"
	"github.com/yourorg/micromart/pkg/tracer"
	"github.com/yourorg/micromart/services/gateway/internal/client"
	"github.com/yourorg/micromart/services/gateway/internal/config"
	"github.com/yourorg/micromart/services/gateway/internal/handler"
)

const version = "0.1.0"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log := logger.New(logger.Config{
		Level:       cfg.Log.Level,
		Format:      cfg.Log.Format,
		ServiceName: "gateway",
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	ctx = logger.WithLogger(ctx, log)

	shutdownTracer, err := tracer.New(ctx, tracer.Config{
		ServiceName:    cfg.Telemetry.ServiceName,
		ServiceVersion: version,
		Endpoint:       cfg.Telemetry.Endpoint,
		SampleRate:     cfg.Telemetry.SampleRate,
	})
	if err != nil {
		return fmt.Errorf("init tracer: %w", err)
	}
	defer shutdownTracer(ctx) //nolint:errcheck

	clients, err := client.New(
		cfg.Services.UserAddr,
		cfg.Services.ProductAddr,
		cfg.Services.SearchAddr,
		cfg.Services.CartAddr,
		cfg.Services.OrderAddr,
	)
	if err != nil {
		return fmt.Errorf("init gRPC clients: %w", err)
	}
	defer clients.Close()

	gin.SetMode(cfg.Server.GinMode)

	h := handler.NewHandlers(clients)
	router := handler.NewRouter(h, cfg.JWT.Secret, cfg.RateLimit.RequestsPerSecond, cfg.RateLimit.Burst, log)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTPPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("gateway starting", "port", cfg.Server.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server error", "error", err)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down gateway")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("HTTP server shutdown: %w", err)
	}

	return nil
}
