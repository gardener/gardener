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
	// DataKeyEncryptionSecretEncoding is the key in a secret data that defines if the secret is already base64 encoded or not.
	// The only possible value is "none". For backwards-compatibility, a missing encoding field is interpreted as "base64" encoding.
	//
	// kube-apiserver's EncryptionConfiguration expects the key secret to be base64 encoded.
	// Previously, a 32-byte key was generated and it was set in the EncryptionConfiguration without being base64 encoded.
	// This resulted in 24-byte key to used by kube-apiserver after decoding the 32-byte key (that was expected to be base64 encoded but it was not).
	// To fix this, new etcd encryption keys are generated with encoding=none. When encoding=none, the key set in the EncryptionConfiguration is base64 encoded.
	// In this way, we make sure to use the same generated 32-byte key.
	// For more information, see https://github.com/gardener/gardener/pull/11150.
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
		KeyName: fmt.Sprintf("key%d", Clock.Now().Unix()),
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
