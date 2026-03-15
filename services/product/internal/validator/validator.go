package validator

import (
	apperrors "github.com/yourorg/micromart/pkg/errors"
)

const (
	maxNameLen        = 255
	maxDescriptionLen = 5000
)

func ValidateName(name string) error {
	if len(name) == 0 || len(name) > maxNameLen {
		return apperrors.NewInvalidInput("name must be between 1 and %d characters", maxNameLen)
	}
	return nil
}

func ValidateDescription(desc string) error {
	if len(desc) > maxDescriptionLen {
		return apperrors.NewInvalidInput("description must not exceed %d characters", maxDescriptionLen)
	}
	return nil
}

func ValidatePrice(price int64) error {
	if price < 0 {
		return apperrors.NewInvalidInput("price must be non-negative")
	}
	return nil
}

func ValidateStock(stock int32) error {
	if stock < 0 {
		return apperrors.NewInvalidInput("stock must be non-negative")
	}
	return nil
}

func ValidateUUID(id, field string) error {
	if id == "" {
		return apperrors.NewInvalidInput("%s is required", field)
	}
	return nil
}

func ValidatePageSize(pageSize int32) int32 {
	if pageSize <= 0 || pageSize > 100 {
		return 20
	}
	return pageSize
}

func ValidatePage(page int32) int32 {
	if page <= 0 {
		return 1
	}
	return page
}
