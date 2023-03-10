package util

func Contains[T comparable](s []T, e T) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
}

func Map[T, V any](s []T, fn func(T) V) []V {
	result := make([]V, len(s))
	for i, t := range s {
		result[i] = fn(t)
	}
	return result
}

func Filter[T any](s []T, fn func(T) bool) []T {
	result := make([]T, 0)
	for _, t := range s {
		if fn(t) {
			result = append(result, t)
		}
	}
	return result
}
