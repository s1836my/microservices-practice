package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"github.com/yourorg/micromart/pkg/grpcserver"
	"github.com/yourorg/micromart/pkg/health"
	"github.com/yourorg/micromart/pkg/logger"
	"github.com/yourorg/micromart/pkg/tracer"
	searchv1 "github.com/yourorg/micromart/proto/search/v1"
	"github.com/yourorg/micromart/services/search/internal/config"
	"github.com/yourorg/micromart/services/search/internal/handler"
	"github.com/yourorg/micromart/services/search/internal/repository"
	"github.com/yourorg/micromart/services/search/internal/service"
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
		ServiceName: "search-service",
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

	repo := repository.NewElasticsearchRepository(cfg.Elasticsearch.URL, cfg.Elasticsearch.Index, &http.Client{
		Timeout: 10 * time.Second,
	})
	if err := repo.EnsureIndex(ctx); err != nil {
		return fmt.Errorf("ensure search index: %w", err)
	}

	svc := service.NewSearchService(repo)
	searchHandler := handler.NewSearchHandler(svc)

	if cfg.Kafka.Enabled {
		processor := service.NewProductEventProcessor(repo, log)
		consumer := service.NewProductConsumer(cfg.Kafka.Brokers, cfg.Kafka.GroupID, cfg.Kafka.Topic, processor, log)
		go consumer.Run(ctx)
	}

	esChecker := health.NewCheckerFunc("elasticsearch", func(hctx context.Context) error {
		return repo.Ping(hctx)
	})
	healthMux := health.NewMux(version, esChecker)
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
		searchv1.RegisterSearchServiceServer(s, searchHandler)
	})
}
