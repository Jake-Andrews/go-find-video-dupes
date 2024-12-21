package utils

func ToInterfaceSlice(ints []int64) []interface{} {
	result := make([]interface{}, len(ints))
	for i, v := range ints {
		result[i] = v
	}
	return result
}
