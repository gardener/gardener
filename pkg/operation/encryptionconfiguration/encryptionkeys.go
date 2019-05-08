package encryptionconfiguration

import (
	"crypto/rand"
	"encoding/base64"
	b64 "encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
)

func createEncryptionKey() (apiserverconfigv1.Key, error) {
	var key apiserverconfigv1.Key
	keyString, err := createKeySecret()
	if err != nil {
		return key, err
	}
	if len(keyString) == 0 {
		return key, fmt.Errorf("generated random encryption key is of unexpected length 0")
	}
	key.Name = createKeyName()
	key.Secret = keyString
	return key, nil
}

func createKeyName() string {
	nanos := time.Now().UnixNano()
	name := ecKeyPrefix + strconv.FormatInt(nanos, 10)
	return name
}

func createKeySecret() (string, error) {
	keyArray := make([]byte, ecKeySecretLen)
	_, err := rand.Read(keyArray)
	if err != nil {
		return "", fmt.Errorf("error while obtaining random data: %v", err)
	}
	sEnc := b64.StdEncoding.EncodeToString(keyArray)
	return sEnc, nil
}

func parseTimestampFromKeyName(keyName string) (int64, error) {
	nameConsistent, err := isKeyNameConsistent(keyName)
	if err != nil {
		return 0, err
	} else if !nameConsistent {
		return 0, nil
	}
	timestampstring := keyName[len(ecKeyPrefix):len(keyName)]
	timestamp, _ := strconv.ParseInt(timestampstring, 10, 64)
	return timestamp, nil
}

func isKeyConsistent(key apiserverconfigv1.Key) (bool, error) {
	nameConsistent, err := isKeyNameConsistent(key.Name)
	if err != nil {
		return false, err
	} else if !nameConsistent {
		return false, nil
	}
	secretConsistent, err := isKeySecretConsistent(key.Secret)
	if err != nil {
		return false, err
	} else if !secretConsistent {
		return false, nil
	}
	return true, nil
}

func isKeyNameConsistent(keyName string) (bool, error) {
	if !strings.HasPrefix(keyName, ecKeyPrefix) {
		return false, fmt.Errorf("unexpected prefix (%v) for key name found", keyName)
	}
	timestampstring := keyName[len(ecKeyPrefix):len(keyName)]
	timestamp, err := strconv.ParseInt(timestampstring, 10, 64)
	if err != nil {
		return false, fmt.Errorf("error occurred when parsing timestamp from key name (%v): %v", timestampstring, err)
	}
	if timestamp < ecKeyTimestampMaxAgeNanos {
		return false, fmt.Errorf("timestamp (%v) of key ought to be UnixNano but is too old", timestampstring)
	}
	nowNanos := time.Now().UnixNano()
	if timestamp > nowNanos+ecKeyTimestampMaxClockSkewNanos {
		return false, fmt.Errorf("timestamp (%v) of key is more than a day larger than current time (%v)", timestampstring, nowNanos)
	}
	return true, nil
}

func isKeySecretConsistent(keySecret string) (bool, error) {
	if len(keySecret) == 0 {
		return false, fmt.Errorf("key secret cannot be empty")
	}
	keyArray, err := base64.StdEncoding.DecodeString(keySecret)
	if err != nil {
		return false, fmt.Errorf("key secret not valid b64: %v", err)
	}
	l := len(keyArray)
	if l < ecKeySecretLen {
		return false, fmt.Errorf("key secret length (%v) too short and ought to be (%v) bytes", l, ecKeySecretLen)
	}
	return true, nil
}
