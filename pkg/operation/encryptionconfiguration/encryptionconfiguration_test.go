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

// kind: EncryptionConfiguration
// apiVersion: apiserver.config.k8s.io/v1
// resources:
//   - resources:
//     - secrets
//     providers:
//     - aescbc:
//         keys:
//         - name: key1553679720
//           secret: t44dGAwGt73RMOSNwp4Z9QXadtnLvC4fZWgzS8Tjz+c=
//     - identity: {}
func TestCreateToYAMLFromYAML(t *testing.T) {
	ec, err := CreateNewPassiveConfiguration()
	if err != nil {
		t.Fatalf("error during CreateNewPassiveConfiguration: %v", err)
	}
	str, err := ToYAML(ec)
	if err != nil {
		t.Fatalf("error during YAML creation: %v", err)
	}
	fmt.Println(string(str))
	ec2, err := CreateFromYAML(str)
	if err != nil {
		t.Fatalf("error during object creation from YAML string: %v", err)
	}
	str2, err := ToYAML(ec2)
	if err != nil {
		t.Fatalf("error during second YAML creation: %v", err)
	}
	fmt.Println(string(str2))
}

func TestConsistencyCorrect(t *testing.T) {
	ec, err := CreateNewPassiveConfiguration()
	if err != nil {
		t.Fatalf("error during CreateNewPassiveConfiguration: %v", err)
	}
	ok, err := IsConsistent(ec)
	if err != nil {
		t.Fatalf("error during consistency check: %v", err)
	}
	if !ok {
		t.Fatal("Expected initial configuration to be consistent")
	}
}
