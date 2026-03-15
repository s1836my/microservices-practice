package service

import (
	apperrors "github.com/yourorg/micromart/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

func hashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", apperrors.Wrap(apperrors.CodeInternal, err, "hash password")
	}
	return string(hash), nil
}

func verifyPassword(hash, plain string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)); err != nil {
		return apperrors.NewUnauthorized("invalid credentials")
	}
	return nil
}
