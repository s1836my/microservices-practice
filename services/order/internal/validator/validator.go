package validator

const (
	defaultPage     = int32(1)
	defaultPageSize = int32(20)
	maxPageSize     = int32(100)
)

func ValidatePage(page int32) int32 {
	if page < 1 {
		return defaultPage
	}
	return page
}

func ValidatePageSize(pageSize int32) int32 {
	if pageSize < 1 {
		return defaultPageSize
	}
	if pageSize > maxPageSize {
		return maxPageSize
	}
	return pageSize
}
