package validator

import (
	"regexp"

	"github.com/google/uuid"
	apperrors "github.com/yourorg/micromart/pkg/errors"
)

var emailRegexp = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func ValidateEmail(email string) error {
	if len(email) == 0 || len(email) > 255 {
		return apperrors.NewInvalidInput("email must be between 1 and 255 characters")
	}
	if !emailRegexp.MatchString(email) {
		return apperrors.NewInvalidInput("invalid email format")
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < 8 {
		return apperrors.NewInvalidInput("password must be at least 8 characters")
	}
	if len(password) > 72 {
		return apperrors.NewInvalidInput("password must not exceed 72 characters")
	}
	return nil
}

func ValidateName(name string) error {
	if len(name) == 0 || len(name) > 100 {
		return apperrors.NewInvalidInput("name must be between 1 and 100 characters")
	}
	return nil
}

func ValidateUUID(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return apperrors.NewInvalidInput("invalid uuid: %s", id)
	}
	return nil
}
