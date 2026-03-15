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

	userv1 "github.com/yourorg/micromart/proto/user/v1"
	"github.com/yourorg/micromart/pkg/grpcserver"
	"github.com/yourorg/micromart/pkg/health"
	"github.com/yourorg/micromart/pkg/logger"
	"github.com/yourorg/micromart/pkg/tracer"
	"github.com/yourorg/micromart/services/user/internal/config"
	"github.com/yourorg/micromart/services/user/internal/handler"
	"github.com/yourorg/micromart/services/user/internal/repository"
	"github.com/yourorg/micromart/services/user/internal/service"
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
		ServiceName: "user-service",
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

	pool, err := pgxpool.New(ctx, cfg.Database.DSN())
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	userRepo := repository.NewUserRepository(pool)
	refreshTokenRepo := repository.NewRefreshTokenRepository(pool)

	tokenSvc := service.NewTokenService(
		cfg.JWT.Secret,
		cfg.JWT.AccessTTL,
		cfg.JWT.RefreshTTL,
		refreshTokenRepo,
	)
	userSvc := service.NewUserService(userRepo, tokenSvc)
	userHandler := handler.NewUserHandler(userSvc)

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

	// ctx キャンセル時にヘルスチェックサーバーも graceful shutdown
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
		userv1.RegisterUserServiceServer(s, userHandler)
	})
}
