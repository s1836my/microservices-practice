package errors

import (
	"errors"
	"fmt"
)

// Code はアプリケーション共通エラーコード。
type Code int

const (
	CodeUnknown          Code = 0
	CodeNotFound         Code = 1
	CodeAlreadyExists    Code = 2
	CodeInvalidInput     Code = 3
	CodeUnauthorized     Code = 4
	CodePermissionDenied Code = 5
	CodeInternal         Code = 6
	CodeUnavailable      Code = 7
)

// AppError はアプリケーション共通エラー型。
type AppError struct {
	Code    Code
	Message string
	cause   error
}

func (e *AppError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.cause)
	}
	return e.Message
}

// Unwrap は errors.As / errors.Is との互換性を提供する。
func (e *AppError) Unwrap() error {
	return e.cause
}

func newAppError(code Code, msg string, args []any) *AppError {
	message := msg
	if len(args) > 0 {
		message = fmt.Sprintf(msg, args...)
	}
	return &AppError{Code: code, Message: message}
}

func NewNotFound(msg string, args ...any) *AppError {
	return newAppError(CodeNotFound, msg, args)
}

func NewAlreadyExists(msg string, args ...any) *AppError {
	return newAppError(CodeAlreadyExists, msg, args)
}

func NewInvalidInput(msg string, args ...any) *AppError {
	return newAppError(CodeInvalidInput, msg, args)
}

func NewUnauthorized(msg string, args ...any) *AppError {
	return newAppError(CodeUnauthorized, msg, args)
}

func NewPermissionDenied(msg string, args ...any) *AppError {
	return newAppError(CodePermissionDenied, msg, args)
}

func NewInternal(msg string, args ...any) *AppError {
	return newAppError(CodeInternal, msg, args)
}

func NewUnavailable(msg string, args ...any) *AppError {
	return newAppError(CodeUnavailable, msg, args)
}

// Wrap は既存エラーを AppError でラップする。
func Wrap(code Code, err error, msg string) *AppError {
	return &AppError{Code: code, Message: msg, cause: err}
}

// AsAppError は error から *AppError を取り出す。
func AsAppError(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}
