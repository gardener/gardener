package encryptionconfiguration

import "sort"

func slicesContainSameElements(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	// copy slices due to sorting
	var copyA = make([]string, len(a))
	copy(copyA, a)
	var copyB = make([]string, len(b))
	copy(copyB, b)
	// Sort slices
	sort.Strings(copyA)
	sort.Strings(copyB)
	for i, v := range copyA {
		if v != copyB[i] {
			return false
		}
	}
	return true
}
