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
	"fmt"

	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/infodata"
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

// GenerateInfoData implements ConfigInterface.
func (s *RSASecretConfig) GenerateInfoData() (infodata.InfoData, error) {
	privateKey, err := generateRSAPrivateKey(s.Bits)
	if err != nil {
		return nil, err
	}

	return NewPrivateKeyInfoData(utils.EncodePrivateKey(privateKey)), nil
}

// GenerateFromInfoData implements ConfigInterface
func (s *RSASecretConfig) GenerateFromInfoData(infoData infodata.InfoData) (Interface, error) {
	data, ok := infoData.(*PrivateKeyInfoData)
	if !ok {
		return nil, fmt.Errorf("could not convert InfoData entry %s to RSAPrivateKeyInfoData", s.Name)
	}

	privateKey, err := utils.DecodePrivateKey(data.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("could not load privateKey secret %s: %v", s.Name, err)
	}

	return s.generateWithPrivateKey(privateKey)
}

// LoadFromSecretData implements infodata.Loader
func (s *RSASecretConfig) LoadFromSecretData(secretData map[string][]byte) (infodata.InfoData, error) {
	privateKey := secretData[DataKeyRSAPrivateKey]
	return NewPrivateKeyInfoData(privateKey), nil
}

// GenerateRSAKeys computes a RSA private key based on the configured number of bits.
func (s *RSASecretConfig) GenerateRSAKeys() (*RSAKeys, error) {
	privateKey, err := generateRSAPrivateKey(s.Bits)
	if err != nil {
		return nil, err
	}

	return s.generateWithPrivateKey(privateKey)
}

func (s *RSASecretConfig) generateWithPrivateKey(privateKey *rsa.PrivateKey) (*RSAKeys, error) {
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
