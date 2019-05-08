package encryptionconfiguration

import (
	"crypto/rand"
	b64 "encoding/base64"
	"testing"
	"time"
)

func TestEncryptionKeyCreation(t *testing.T) {
	key, err := createEncryptionKey()
	if err != nil {
		t.Fatalf("error during key generation: %v", err)
	}
	ok, err := isKeyConsistent(key)
	if (err != nil) || (!ok) {
		t.Fatalf("newly generated key ought to be consistent: %v", err)
	}
}

func TestEncryptionKeyInconsistentNameDetection(t *testing.T) {
	key, err := createEncryptionKey()
	if err != nil {
		t.Fatalf("error during key generation: %v", err)
	}
	key.Name = "wrongkey"
	ok, err := isKeyConsistent(key)
	if err == nil || ok {
		t.Fatalf("expected inconcistency (wrong prefix) of key name (%v) not detected", key.Name)
	}
	key.Name = "key"
	ok, err = isKeyConsistent(key)
	if err == nil || ok {
		t.Fatalf("expected inconcistency (missing timestamp) of key name (%v) not detected", key.Name)
	}
	key.Name = "key0"
	ok, err = isKeyConsistent(key)
	if err == nil || ok {
		t.Fatalf("expected inconcistency (wrong timestamp, no nanos) of key name (%v) not detected", key.Name)
	}
	key.Name = "key1"
	ok, err = isKeyConsistent(key)
	if err == nil || ok {
		t.Fatalf("expected inconcistency (wrong timestamp, no nanos) of key name (%v) not detected", key.Name)
	}
	key.Name = "key0000000000000000000"
	ok, err = isKeyConsistent(key)
	if err == nil || ok {
		t.Fatalf("expected inconcistency (wrong timestamp, too old) of key name (%v) not detected", key.Name)
	}
	key.Name = "key32503676400000000000"
	ok, err = isKeyConsistent(key)
	if err == nil || ok {
		t.Fatalf("expected inconcistency (wrong timestamp, far in future) of key name (%v) not detected", key.Name)
	}
}

func TestEncryptionKeyInconsistentSecretDetection(t *testing.T) {
	key, err := createEncryptionKey()
	if err != nil {
		t.Fatalf("error during key generation: %v", err)
	}
	key.Secret = ""
	ok, err := isKeyConsistent(key)
	if err == nil || ok {
		t.Fatalf("expected inconcistency (empty secret) of key secret (%v) not detected", key.Secret)
	}
	key.Secret = "secret$"
	ok, err = isKeyConsistent(key)
	if err == nil || ok {
		t.Fatalf("expected inconcistency (non-base64 characters) of key secret (%v) not detected", key.Secret)
	}
	keyArrayShort := make([]byte, ecKeySecretLen-1)
	_, err = rand.Read(keyArrayShort)
	if err != nil {
		t.Skipf("random data could not be generated for test: %v", err)
	}
	key.Secret = b64.StdEncoding.EncodeToString(keyArrayShort)
	ok, err = isKeyConsistent(key)
	if err == nil || ok {
		t.Fatalf("expected inconcistency (too short) of key secret (%v) not detected", key.Secret)
	}
}
func TestEncryptionKeyNameParsing(t *testing.T) {
	keyName := createKeyName()
	timestamp, err := parseTimestampFromKeyName(keyName)
	if err != nil {
		t.Fatalf("error during timestamp parsing: %v", err)
	}
	nanos := time.Now().UnixNano()
	if (timestamp <= 0) || (timestamp > nanos) {
		t.Fatalf("unexpected timestamp (%v) in key name. current time in nanos (%v)", timestamp, nanos)
	}
}
