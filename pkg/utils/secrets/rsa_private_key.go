// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"

	"golang.org/x/crypto/ssh"

	"github.com/gardener/gardener/pkg/utils"
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
func (s *RSASecretConfig) Generate() (DataInterface, error) {
	privateKey, err := GenerateKey(rand.Reader, s.Bits)
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
