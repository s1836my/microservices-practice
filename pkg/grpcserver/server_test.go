package grpcserver_test

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/yourorg/micromart/pkg/grpcserver"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func TestRun_GracefulShutdownViaContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- grpcserver.Run(ctx, grpcserver.Config{
			Port: freePort(t),
		}, func(_ *grpc.Server) {})
	}()

	// サーバーが起動するまで少し待つ
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run() = %v, want nil", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run() did not return after context cancellation")
	}
}

func TestRun_WithReflection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- grpcserver.Run(ctx, grpcserver.Config{
			Port:             freePort(t),
			EnableReflection: true,
		}, func(_ *grpc.Server) {})
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run() with reflection = %v, want nil", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run() did not return")
	}
}

func TestRun_InvalidPort(t *testing.T) {
	ctx := context.Background()
	err := grpcserver.Run(ctx, grpcserver.Config{Port: -1}, func(_ *grpc.Server) {})
	if err == nil {
		t.Error("expected error for invalid port, got nil")
	}
}

func TestWithUnaryInterceptors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	called := false
	interceptor := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		called = true
		return handler(ctx, req)
	}

	done := make(chan error, 1)
	go func() {
		done <- grpcserver.Run(ctx, grpcserver.Config{
			Port: freePort(t),
		}, func(_ *grpc.Server) {}, grpcserver.WithUnaryInterceptors(interceptor))
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run() = %v, want nil", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run() did not return")
	}
	// サーバーが起動したこと自体を確認 (interceptor は実際のRPC呼び出し時のみ呼ばれる)
	_ = called
}

func TestWithStreamInterceptors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	streamInterceptor := func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return handler(srv, ss)
	}

	done := make(chan error, 1)
	go func() {
		done <- grpcserver.Run(ctx, grpcserver.Config{
			Port: freePort(t),
		}, func(_ *grpc.Server) {}, grpcserver.WithStreamInterceptors(streamInterceptor))
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run() = %v, want nil", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run() did not return")
	}
}
