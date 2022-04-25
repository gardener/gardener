// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
