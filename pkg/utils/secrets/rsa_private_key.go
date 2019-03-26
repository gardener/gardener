// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"bytes"
	"crypto/rand"
	"crypto/rsa"

	"github.com/gardener/gardener/pkg/utils"
	"golang.org/x/crypto/ssh"
)

const (
	// DataKeyRSAPrivateKey is the key in a secret data holding the RSA private key.
	DataKeyRSAPrivateKey = "id_rsa"
	// DataKeySSHAuthorizedKeys is the key in a secret data holding the OpenSSH authorized keys.
	DataKeySSHAuthorizedKeys = "id_rsa.pub"
)

// RSASecretConfig containing information about the number of bits which should be used for the to-be-created RSA private key.
type RSASecretConfig struct {
	Name string

	Bits       int
	UsedForSSH bool
}

// RSAKeys contains the private key, the public key, and optionally the OpenSSH-formatted authorized keys file data.
type RSAKeys struct {
	Name string

	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey

	OpenSSHAuthorizedKey []byte
}

// GetName returns the name of the secret.
func (s *RSASecretConfig) GetName() string {
	return s.Name
}

// Generate implements ConfigInterface.
func (s *RSASecretConfig) Generate() (Interface, error) {
	return s.GenerateRSAKeys()
}

// GenerateRSAKeys computes a RSA private key based on the configured number of bits.
func (s *RSASecretConfig) GenerateRSAKeys() (*RSAKeys, error) {
	privateKey, err := generateRSAPrivateKey(s.Bits)
	if err != nil {
		return nil, err
	}

	rsa := &RSAKeys{
		Name: s.Name,

		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
	}

	if s.UsedForSSH {
		sshPublicKey, err := generateSSHAuthorizedKeys(rsa.PrivateKey)
		if err != nil {
			return nil, err
		}
		rsa.OpenSSHAuthorizedKey = sshPublicKey
	}

	return rsa, nil
}

// SecretData computes the data map which can be used in a Kubernetes secret.
func (r *RSAKeys) SecretData() map[string][]byte {
	data := map[string][]byte{
		DataKeyRSAPrivateKey: utils.EncodePrivateKey(r.PrivateKey),
	}

	if r.OpenSSHAuthorizedKey != nil {
		data[DataKeySSHAuthorizedKeys] = r.OpenSSHAuthorizedKey
	}

	return data
}

// generateRSAPrivateKey generates a RSA private for the given number of <bits>.
func generateRSAPrivateKey(bits int) (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, bits)
}

// generateSSHAuthorizedKeys takes a RSA private key <privateKey> and generates the corresponding public key.
// It serializes the public key for inclusion in an OpenSSH `authorized_keys` file and it trims the new-
// line at the end.
func generateSSHAuthorizedKeys(privateKey *rsa.PrivateKey) ([]byte, error) {
	pubKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, err
	}
	publicKey := ssh.MarshalAuthorizedKey(pubKey)
	return bytes.Trim(publicKey, "\x0a"), nil
}
