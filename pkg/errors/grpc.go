package errors

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var codeToGRPC = map[Code]codes.Code{
	CodeUnknown:          codes.Unknown,
	CodeNotFound:         codes.NotFound,
	CodeAlreadyExists:    codes.AlreadyExists,
	CodeInvalidInput:     codes.InvalidArgument,
	CodeUnauthorized:     codes.Unauthenticated,
	CodePermissionDenied: codes.PermissionDenied,
	CodeInternal:         codes.Internal,
	CodeUnavailable:      codes.Unavailable,
}

var grpcToCode = map[codes.Code]Code{
	codes.Unknown:          CodeUnknown,
	codes.NotFound:         CodeNotFound,
	codes.AlreadyExists:    CodeAlreadyExists,
	codes.InvalidArgument:  CodeInvalidInput,
	codes.Unauthenticated:  CodeUnauthorized,
	codes.PermissionDenied: CodePermissionDenied,
	codes.Internal:         CodeInternal,
	codes.Unavailable:      CodeUnavailable,
}

// ToGRPCStatus は error を gRPC *status.Status に変換する。
// AppError でなければ codes.Internal として扱う。
func ToGRPCStatus(err error) *status.Status {
	if err == nil {
		return status.New(codes.OK, "")
	}
	appErr, ok := AsAppError(err)
	if !ok {
		return status.New(codes.Internal, err.Error())
	}
	grpcCode, ok := codeToGRPC[appErr.Code]
	if !ok {
		grpcCode = codes.Internal
	}
	return status.New(grpcCode, appErr.Message)
}

// FromGRPCStatus は gRPC *status.Status を AppError に変換する。
func FromGRPCStatus(s *status.Status) *AppError {
	appCode, ok := grpcToCode[s.Code()]
	if !ok {
		appCode = CodeUnknown
	}
	return &AppError{Code: appCode, Message: s.Message()}
}
