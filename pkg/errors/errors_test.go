package errors_test

import (
	"errors"
	"fmt"
	"testing"

	apperrors "github.com/yourorg/micromart/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNewErrors(t *testing.T) {
	cases := []struct {
		name    string
		err     *apperrors.AppError
		code    apperrors.Code
		message string
	}{
		{"NotFound", apperrors.NewNotFound("user not found"), apperrors.CodeNotFound, "user not found"},
		{"AlreadyExists", apperrors.NewAlreadyExists("email taken"), apperrors.CodeAlreadyExists, "email taken"},
		{"InvalidInput", apperrors.NewInvalidInput("invalid email"), apperrors.CodeInvalidInput, "invalid email"},
		{"Unauthorized", apperrors.NewUnauthorized("bad token"), apperrors.CodeUnauthorized, "bad token"},
		{"PermissionDenied", apperrors.NewPermissionDenied("forbidden"), apperrors.CodePermissionDenied, "forbidden"},
		{"Internal", apperrors.NewInternal("db error"), apperrors.CodeInternal, "db error"},
		{"Unavailable", apperrors.NewUnavailable("service down"), apperrors.CodeUnavailable, "service down"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err.Code != tc.code {
				t.Errorf("Code = %v, want %v", tc.err.Code, tc.code)
			}
			if tc.err.Message != tc.message {
				t.Errorf("Message = %q, want %q", tc.err.Message, tc.message)
			}
			if tc.err.Error() != tc.message {
				t.Errorf("Error() = %q, want %q", tc.err.Error(), tc.message)
			}
		})
	}
}

func TestNewErrors_WithFormat(t *testing.T) {
	err := apperrors.NewNotFound("user %d not found", 42)
	want := "user 42 not found"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestWrap(t *testing.T) {
	cause := fmt.Errorf("connection refused")
	err := apperrors.Wrap(apperrors.CodeUnavailable, cause, "database unavailable")

	if err.Code != apperrors.CodeUnavailable {
		t.Errorf("Code = %v, want CodeUnavailable", err.Code)
	}
	if !errors.Is(err, cause) {
		t.Error("Wrap should preserve cause via errors.Is")
	}
	if err.Error() == "database unavailable" {
		// Error() should include cause
	}
}

func TestAsAppError(t *testing.T) {
	original := apperrors.NewNotFound("not found")

	got, ok := apperrors.AsAppError(original)
	if !ok {
		t.Fatal("AsAppError should return true for AppError")
	}
	if got.Code != apperrors.CodeNotFound {
		t.Errorf("Code = %v, want CodeNotFound", got.Code)
	}
}

func TestAsAppError_Wrapped(t *testing.T) {
	inner := apperrors.NewNotFound("not found")
	wrapped := fmt.Errorf("outer: %w", inner)

	got, ok := apperrors.AsAppError(wrapped)
	if !ok {
		t.Fatal("AsAppError should unwrap to find AppError")
	}
	if got.Code != apperrors.CodeNotFound {
		t.Errorf("Code = %v, want CodeNotFound", got.Code)
	}
}

func TestAsAppError_NonAppError(t *testing.T) {
	_, ok := apperrors.AsAppError(fmt.Errorf("plain error"))
	if ok {
		t.Error("AsAppError should return false for non-AppError")
	}
}

func TestToGRPCStatus(t *testing.T) {
	cases := []struct {
		err      error
		wantCode codes.Code
	}{
		{nil, codes.OK},
		{apperrors.NewNotFound("not found"), codes.NotFound},
		{apperrors.NewAlreadyExists("exists"), codes.AlreadyExists},
		{apperrors.NewInvalidInput("invalid"), codes.InvalidArgument},
		{apperrors.NewUnauthorized("unauth"), codes.Unauthenticated},
		{apperrors.NewPermissionDenied("forbidden"), codes.PermissionDenied},
		{apperrors.NewInternal("internal"), codes.Internal},
		{apperrors.NewUnavailable("unavailable"), codes.Unavailable},
		{fmt.Errorf("plain error"), codes.Internal},
	}

	for _, tc := range cases {
		s := apperrors.ToGRPCStatus(tc.err)
		if s.Code() != tc.wantCode {
			t.Errorf("ToGRPCStatus(%v).Code() = %v, want %v", tc.err, s.Code(), tc.wantCode)
		}
	}
}

func TestFromGRPCStatus(t *testing.T) {
	cases := []struct {
		grpcCode codes.Code
		wantCode apperrors.Code
	}{
		{codes.NotFound, apperrors.CodeNotFound},
		{codes.AlreadyExists, apperrors.CodeAlreadyExists},
		{codes.InvalidArgument, apperrors.CodeInvalidInput},
		{codes.Unauthenticated, apperrors.CodeUnauthorized},
		{codes.Internal, apperrors.CodeInternal},
	}

	for _, tc := range cases {
		s := status.New(tc.grpcCode, "test message")
		appErr := apperrors.FromGRPCStatus(s)
		if appErr.Code != tc.wantCode {
			t.Errorf("FromGRPCStatus(%v).Code = %v, want %v", tc.grpcCode, appErr.Code, tc.wantCode)
		}
		if appErr.Message != "test message" {
			t.Errorf("Message = %q, want %q", appErr.Message, "test message")
		}
	}
}

func TestRoundTrip(t *testing.T) {
	original := apperrors.NewNotFound("user 123 not found")

	grpcStatus := apperrors.ToGRPCStatus(original)
	recovered := apperrors.FromGRPCStatus(grpcStatus)

	if recovered.Code != original.Code {
		t.Errorf("round-trip Code = %v, want %v", recovered.Code, original.Code)
	}
	if recovered.Message != original.Message {
		t.Errorf("round-trip Message = %q, want %q", recovered.Message, original.Message)
	}
}
