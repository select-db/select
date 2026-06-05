package toolkit

import "slices"

func Intersects[T comparable](a []T, b []T) bool {
	return len(Intersection(a, b)) > 0
}

func Intersection[T comparable](a []T, b []T) []T {
	set := make([]T, 0)

	for _, v := range a {
		if slices.Contains(b, v) {
			set = append(set, v)
		}
	}

	return set
}
