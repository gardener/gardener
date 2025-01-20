// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"fmt"

	"github.com/gardener/gardener/pkg/utils"
)

const (
	// DataKeyEncryptionKeyName is the key in a secret data holding the key.
	DataKeyEncryptionKeyName = "key"
	// DataKeyEncryptionSecret is the key in a secret data holding the secret.
	DataKeyEncryptionSecret = "secret"
	// DataKeyEncryptionSecretEncoding is the key in a secret data that defines if the secret is already base64 encoded or not.
	// Possible values are "base64" and "none".
	DataKeyEncryptionSecretEncoding = "encoding"
)

// ETCDEncryptionKeySecretConfig contains the specification for a to-be-generated random key.
type ETCDEncryptionKeySecretConfig struct {
	Name         string
	SecretLength int
}

// ETCDEncryptionKey contains the generated key.
type ETCDEncryptionKey struct {
	KeyName string
	Secret  []byte
}

// GetName returns the name of the secret.
func (s *ETCDEncryptionKeySecretConfig) GetName() string {
	return s.Name
}

// Generate implements ConfigInterface.
func (s *ETCDEncryptionKeySecretConfig) Generate() (DataInterface, error) {
	key := make([]byte, s.SecretLength)
	_, err := Read(key)
	if err != nil {
		return nil, err
	}

	return &ETCDEncryptionKey{
		KeyName: fmt.Sprintf("key%d-%s", Clock.Now().Unix(), utils.ComputeSHA256Hex(key)[:6]),
		Secret:  key,
	}, nil
}

// SecretData computes the data map which can be used in a Kubernetes secret.
func (b *ETCDEncryptionKey) SecretData() map[string][]byte {
	return map[string][]byte{
		DataKeyEncryptionKeyName:        []byte(b.KeyName),
		DataKeyEncryptionSecret:         b.Secret,
		DataKeyEncryptionSecretEncoding: []byte("none"),
	}
}
