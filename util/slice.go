package util

// Difference returns the items present in a, that
// are not found in b. E.g.
// E.g. Difference([1,2,3], [1,2]) = [3]
func Difference(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}

	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}

	return diff
}
