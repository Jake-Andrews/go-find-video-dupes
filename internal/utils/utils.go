package utils

func ToInterfaceSlice(ints []int64) []any {
	result := make([]any, len(ints))
	for i, v := range ints {
		result[i] = v
	}
	return result
}
