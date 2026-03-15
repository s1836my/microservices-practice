package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apperrors "github.com/yourorg/micromart/pkg/errors"
)

func TestHashPassword(t *testing.T) {
	hash, err := hashPassword("securepassword")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, "securepassword", hash)
}

func TestHashPassword_DifferentHashes(t *testing.T) {
	hash1, err := hashPassword("same")
	require.NoError(t, err)
	hash2, err := hashPassword("same")
	require.NoError(t, err)
	// bcrypt はソルトが異なるため同じ入力でもハッシュが異なる
	assert.NotEqual(t, hash1, hash2)
}

func TestVerifyPassword_Correct(t *testing.T) {
	hash, err := hashPassword("mypassword")
	require.NoError(t, err)
	assert.NoError(t, verifyPassword(hash, "mypassword"))
}

func TestVerifyPassword_Wrong(t *testing.T) {
	hash, err := hashPassword("mypassword")
	require.NoError(t, err)

	err = verifyPassword(hash, "wrongpassword")
	assert.Error(t, err)
	var appErr *apperrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnauthorized, appErr.Code)
}
