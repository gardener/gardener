package encryptionconfiguration

import (
	"fmt"
	"testing"
)

func TestSliceElementCompare(t *testing.T) {
	var s1 = []string{
		"test",
		"Test",
		"test",
		"Test",
		"A",
		"C",
		"B",
	}
	var s2 = []string{
		"Test",
		"test",
		"C",
		"test",
		"Test",
		"A",
		"B",
	}
	if !slicesContainSameElements(s1, s2) {
		t.Fatalf("slices should contain same elements")
	}
	fmt.Println(s1)
	fmt.Println(s2)
}
