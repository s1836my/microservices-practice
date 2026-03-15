package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"

	productv1 "github.com/yourorg/micromart/proto/product/v1"
	"github.com/yourorg/micromart/pkg/grpcserver"
	"github.com/yourorg/micromart/pkg/health"
	"github.com/yourorg/micromart/pkg/logger"
	"github.com/yourorg/micromart/pkg/tracer"
	"github.com/yourorg/micromart/services/product/internal/config"
	"github.com/yourorg/micromart/services/product/internal/handler"
	"github.com/yourorg/micromart/services/product/internal/repository"
	"github.com/yourorg/micromart/services/product/internal/service"
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
		ServiceName: "product-service",
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
	defer shutdownTracer(ctx)

	pool, err := pgxpool.New(ctx, cfg.Database.DSN())
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	if err = pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	// Select Kafka publisher or noop based on config
	var publisher service.EventPublisher
	if cfg.Kafka.Enabled {
		kp := service.NewKafkaPublisher(cfg.Kafka.Brokers, cfg.Kafka.Topic)
		defer kp.Close() //nolint:errcheck
		publisher = kp
	} else {
		publisher = service.NewNoopPublisher()
	}

	repo := repository.NewProductRepository(pool)
	svc := service.NewProductService(repo)
	productHandler := handler.NewProductHandler(svc)

	relay := service.NewOutboxRelay(repo, publisher, log)
	go relay.Run(ctx)

	dbChecker := health.NewCheckerFunc("postgres", func(hctx context.Context) error {
		return pool.Ping(hctx)
	})
	healthMux := health.NewMux(version, dbChecker)
	healthSrv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTPPort),
		Handler:      healthMux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		log.Info("health check server starting", "port", cfg.Server.HTTPPort)
		if err := healthSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("health check server failed", "error", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := healthSrv.Shutdown(shutdownCtx); err != nil {
			log.Error("health check server shutdown failed", "error", err)
		}
	}()

	return grpcserver.Run(ctx, grpcserver.Config{
		Port:             cfg.Server.GRPCPort,
		EnableReflection: cfg.Server.EnableReflection,
	}, func(s *grpc.Server) {
		productv1.RegisterProductServiceServer(s, productHandler)
	})
}
