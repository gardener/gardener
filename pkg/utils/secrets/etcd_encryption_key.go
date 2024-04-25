// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"fmt"
)

const (
	// DataKeyEncryptionKeyName is the key in a secret data holding the key.
	DataKeyEncryptionKeyName = "key"
	// DataKeyEncryptionSecret is the key in a secret data holding the secret.
	DataKeyEncryptionSecret = "secret"
)

// ETCDEncryptionKeySecretConfig contains the specification for a to-be-generated random key.
type ETCDEncryptionKeySecretConfig struct {
	Name         string
	SecretLength int
}

// ETCDEncryptionKey contains the generated key.
type ETCDEncryptionKey struct {
	Name   string
	Key    string
	Secret string
}

// GetName returns the name of the secret.
func (s *ETCDEncryptionKeySecretConfig) GetName() string {
	return s.Name
}

// Generate implements ConfigInterface.
func (s *ETCDEncryptionKeySecretConfig) Generate() (DataInterface, error) {
	secret, err := GenerateRandomString(s.SecretLength)
	if err != nil {
		return nil, err
	}

	return &ETCDEncryptionKey{
		Name:   s.Name,
		Key:    fmt.Sprintf("key%d", Clock.Now().Unix()),
		Secret: secret,
	}, nil
}

// SecretData computes the data map which can be used in a Kubernetes secret.
func (b *ETCDEncryptionKey) SecretData() map[string][]byte {
	return map[string][]byte{
		DataKeyEncryptionKeyName: []byte(b.Key),
		DataKeyEncryptionSecret:  []byte(b.Secret),
	}
}
