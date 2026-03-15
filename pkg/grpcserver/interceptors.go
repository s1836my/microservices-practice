package grpcserver

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apperrors "github.com/yourorg/micromart/pkg/errors"
	"github.com/yourorg/micromart/pkg/logger"
)

// LoggingUnaryInterceptor は slog を使って Unary RPC のリクエスト / レスポンスをログ出力する。
func LoggingUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		start := time.Now()
		l := logger.FromContext(ctx)

		resp, err := handler(ctx, req)

		code := codes.OK
		if err != nil {
			if s, ok := status.FromError(err); ok {
				code = s.Code()
			} else {
				code = codes.Internal
			}
		}

		attrs := []any{
			"method", info.FullMethod,
			"code", code.String(),
			"duration_ms", time.Since(start).Milliseconds(),
		}
		if err != nil {
			attrs = append(attrs, slog.String("error", err.Error()))
			l.WarnContext(ctx, "gRPC request", attrs...)
		} else {
			l.InfoContext(ctx, "gRPC request", attrs...)
		}

		return resp, err
	}
}

// RecoveryUnaryInterceptor は Unary RPC 内の panic を codes.Internal エラーに変換する。
func RecoveryUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				l := logger.FromContext(ctx)
				l.ErrorContext(ctx, "gRPC panic recovered",
					"method", info.FullMethod,
					"panic", r,
					"stack", string(debug.Stack()),
				)
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

// ErrorMappingUnaryInterceptor は pkg/errors.AppError を gRPC status エラーに変換する。
func ErrorMappingUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}
		// すでに gRPC status エラーならそのまま返す
		if _, ok := status.FromError(err); ok {
			return resp, err
		}
		// AppError → gRPC status に変換
		return resp, apperrors.ToGRPCStatus(err).Err()
	}
}
