package utils

func Ptr[T any](value T) *T {
	return &value
}

func DefaultIfNil[T any](ptr *T, defaultValue T) *T {
	if ptr == nil {
		return &defaultValue
	}
	return ptr
}
