package validator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apperrors "github.com/yourorg/micromart/pkg/errors"
	"github.com/yourorg/micromart/services/product/internal/validator"
)

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid name", "Widget Pro", false},
		{"single char", "A", false},
		{"empty", "", true},
		{"too long", string(make([]byte, 256)), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				var appErr *apperrors.AppError
				assert.ErrorAs(t, err, &appErr)
				assert.Equal(t, apperrors.CodeInvalidInput, appErr.Code)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateDescription(t *testing.T) {
	assert.NoError(t, validator.ValidateDescription(""))
	assert.NoError(t, validator.ValidateDescription("A nice product."))
	assert.Error(t, validator.ValidateDescription(string(make([]byte, 5001))))
}

func TestValidatePrice(t *testing.T) {
	assert.NoError(t, validator.ValidatePrice(0))
	assert.NoError(t, validator.ValidatePrice(9999))
	assert.Error(t, validator.ValidatePrice(-1))
}

func TestValidateStock(t *testing.T) {
	assert.NoError(t, validator.ValidateStock(0))
	assert.NoError(t, validator.ValidateStock(100))
	assert.Error(t, validator.ValidateStock(-1))
}

func TestValidatePageSize(t *testing.T) {
	assert.Equal(t, int32(20), validator.ValidatePageSize(0))
	assert.Equal(t, int32(20), validator.ValidatePageSize(-1))
	assert.Equal(t, int32(20), validator.ValidatePageSize(101))
	assert.Equal(t, int32(50), validator.ValidatePageSize(50))
}

func TestValidatePage(t *testing.T) {
	assert.Equal(t, int32(1), validator.ValidatePage(0))
	assert.Equal(t, int32(1), validator.ValidatePage(-1))
	assert.Equal(t, int32(5), validator.ValidatePage(5))
}
