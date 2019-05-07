package encryptionconfiguration

import (
	"crypto/rand"
	b64 "encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
)

func createEncryptionKey() (apiserverconfigv1.Key, error) {
	var key apiserverconfigv1.Key
	nanos := time.Now().UnixNano()
	keyString, err := createNewRandomKeyString()
	if err != nil {
		return key, err
	}
	if len(keyString) == 0 {
		return key, fmt.Errorf("generated random encryption key is of unexpected length 0")
	}
	key.Name = ecKeyPrefix + strconv.FormatInt(nanos, 10)
	key.Secret = keyString
	return key, nil
}

func createNewRandomKeyString() (string, error) {
	keyArray := make([]byte, 32)
	_, err := rand.Read(keyArray)
	if err != nil {
		return "", fmt.Errorf("error while obtaining random data: %v", err)
	}
	sEnc := b64.StdEncoding.EncodeToString(keyArray)
	return sEnc, nil
}

func parseKeyTimestamp(key apiserverconfigv1.Key) (int64, error) {
	if !strings.HasPrefix(key.Name, ecKeyPrefix) {
		return 0, fmt.Errorf("unexpected prefix for key name of key (%v) found", key.Name)
	}
	timestampstring := key.Name[len(ecKeyPrefix):len(key.Name)]
	fmt.Println(timestampstring) // todome remove

	return 0, nil

}
