package encryptionconfiguration

import "testing"

func TestKeyGeneration(t *testing.T) {
	key, err := createNewRandomKeyString()
	if err != nil {
		t.Fatalf("error during key generation: %v", err)
	}
	if len(key) == 0 {
		t.Fatalf("key is of unexpected length 0")
	}
	t.Log(key)
}
