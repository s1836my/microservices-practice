package grpcserver

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/yourorg/micromart/pkg/logger"
)

// Config は gRPC サーバーの設定。
type Config struct {
	Port             int
	EnableReflection bool // grpcurl 等の開発ツール用
}

// RegisterFunc は *grpc.Server にサービスを登録する関数型。
type RegisterFunc func(s *grpc.Server)

type serverOptions struct {
	unaryInterceptors  []grpc.UnaryServerInterceptor
	streamInterceptors []grpc.StreamServerInterceptor
}

// Option は Run のオプション。
type Option func(*serverOptions)

// WithUnaryInterceptors は追加の Unary インターセプターを設定する。
func WithUnaryInterceptors(interceptors ...grpc.UnaryServerInterceptor) Option {
	return func(o *serverOptions) {
		o.unaryInterceptors = append(o.unaryInterceptors, interceptors...)
	}
}

// WithStreamInterceptors は追加の Stream インターセプターを設定する。
func WithStreamInterceptors(interceptors ...grpc.StreamServerInterceptor) Option {
	return func(o *serverOptions) {
		o.streamInterceptors = append(o.streamInterceptors, interceptors...)
	}
}

// Run は gRPC サーバーを起動し、SIGTERM / SIGINT または ctx キャンセルで Graceful Shutdown する。
func Run(ctx context.Context, cfg Config, register RegisterFunc, opts ...Option) error {
	so := &serverOptions{}
	for _, o := range opts {
		o(so)
	}

	// デフォルトインターセプター: Recovery → ErrorMapping → Logging → OTel
	unary := []grpc.UnaryServerInterceptor{
		RecoveryUnaryInterceptor(),
		ErrorMappingUnaryInterceptor(),
		LoggingUnaryInterceptor(),
		otelgrpc.UnaryServerInterceptor(),
	}
	unary = append(unary, so.unaryInterceptors...)

	stream := []grpc.StreamServerInterceptor{
		otelgrpc.StreamServerInterceptor(),
	}
	stream = append(stream, so.streamInterceptors...)

	s := grpc.NewServer(
		grpc.ChainUnaryInterceptor(unary...),
		grpc.ChainStreamInterceptor(stream...),
	)
	register(s)

	if cfg.EnableReflection {
		reflection.Register(s)
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return fmt.Errorf("listen on port %d: %w", cfg.Port, err)
	}

	l := logger.FromContext(ctx)
	l.Info("gRPC server starting", "port", cfg.Port)

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- s.Serve(lis)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(quit)

	select {
	case err := <-serveErr:
		return err
	case <-quit:
		l.Info("shutting down gRPC server (signal)")
	case <-ctx.Done():
		l.Info("shutting down gRPC server (context cancelled)")
	}

	s.GracefulStop()
	return nil
}
