package grpcserver_test

import (
	"context"
	"fmt"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apperrors "github.com/yourorg/micromart/pkg/errors"
	"github.com/yourorg/micromart/pkg/grpcserver"
)

var dummyInfo = &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

func TestRecoveryUnaryInterceptor(t *testing.T) {
	interceptor := grpcserver.RecoveryUnaryInterceptor()

	panicHandler := func(_ context.Context, _ any) (any, error) {
		panic("something went wrong")
	}

	_, err := interceptor(context.Background(), nil, dummyInfo, panicHandler)
	if err == nil {
		t.Fatal("expected error after panic, got nil")
	}

	s, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T", err)
	}
	if s.Code() != codes.Internal {
		t.Errorf("code = %v, want Internal", s.Code())
	}
}

func TestErrorMappingUnaryInterceptor_AppError(t *testing.T) {
	interceptor := grpcserver.ErrorMappingUnaryInterceptor()

	handler := func(_ context.Context, _ any) (any, error) {
		return nil, apperrors.NewNotFound("user not found")
	}

	_, err := interceptor(context.Background(), nil, dummyInfo, handler)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	s, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T", err)
	}
	if s.Code() != codes.NotFound {
		t.Errorf("code = %v, want NotFound", s.Code())
	}
}

func TestErrorMappingUnaryInterceptor_GRPCStatusPassThrough(t *testing.T) {
	interceptor := grpcserver.ErrorMappingUnaryInterceptor()

	want := status.Error(codes.AlreadyExists, "already exists")
	handler := func(_ context.Context, _ any) (any, error) {
		return nil, want
	}

	_, err := interceptor(context.Background(), nil, dummyInfo, handler)
	if err != want {
		t.Errorf("gRPC status error should pass through unchanged")
	}
}

func TestErrorMappingUnaryInterceptor_NoError(t *testing.T) {
	interceptor := grpcserver.ErrorMappingUnaryInterceptor()

	handler := func(_ context.Context, _ any) (any, error) {
		return "response", nil
	}

	resp, err := interceptor(context.Background(), nil, dummyInfo, handler)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if resp != "response" {
		t.Errorf("resp = %v, want %q", resp, "response")
	}
}

func TestLoggingUnaryInterceptor(t *testing.T) {
	interceptor := grpcserver.LoggingUnaryInterceptor()

	t.Run("success", func(t *testing.T) {
		handler := func(_ context.Context, _ any) (any, error) {
			return "ok", nil
		}
		resp, err := interceptor(context.Background(), nil, dummyInfo, handler)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if resp != "ok" {
			t.Errorf("resp = %v, want %q", resp, "ok")
		}
	})

	t.Run("error propagated", func(t *testing.T) {
		want := fmt.Errorf("some error")
		handler := func(_ context.Context, _ any) (any, error) {
			return nil, want
		}
		_, err := interceptor(context.Background(), nil, dummyInfo, handler)
		if err != want {
			t.Errorf("error = %v, want %v", err, want)
		}
	})
}
