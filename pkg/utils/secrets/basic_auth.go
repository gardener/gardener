// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"github.com/gardener/gardener/pkg/utils"
)

type formatType string

const (
	// BasicAuthFormatNormal indicates that the data map should be rendered the normal way (dedicated keys for
	// username and password.
	BasicAuthFormatNormal formatType = "normal"

	// DataKeyUserName is the key in a secret data holding the username.
	DataKeyUserName = "username"
	// DataKeyPassword is the key in a secret data holding the password.
	DataKeyPassword = "password"
	// DataKeyAuth is the key in a secret data holding the basic authentication schemed credentials pair as string.
	DataKeyAuth = "auth"
)

// BasicAuthSecretConfig contains the specification for a to-be-generated basic authentication secret.
type BasicAuthSecretConfig struct {
	Name string
	// Format is the format type.
	//
	// Do not remove this field, even though the field is not used and there is only one supported format ("normal").
	// The secret manager computes the Secret hash based on the config object (BasicAuthSecretConfig). A field removal in the
	// BasicAuthSecretConfig object would compute a new Secret hash and this would lead the existing Secrets to be regenerated.
	// Hence, usages of the BasicAuthSecretConfig should continue to pass the Format field with value "normal".
	Format formatType

	Username       string
	PasswordLength int
}

// BasicAuth contains the username, the password and optionally hash of the password.
type BasicAuth struct {
	Name string

	Username string
	Password string
	auth     []byte
}

// GetName returns the name of the secret.
func (s *BasicAuthSecretConfig) GetName() string {
	return s.Name
}

// Generate implements ConfigInterface.
func (s *BasicAuthSecretConfig) Generate() (DataInterface, error) {
	password, err := GenerateRandomString(s.PasswordLength)
	if err != nil {
		return nil, err
	}

	auth, err := utils.CreateBcryptCredentials([]byte(s.Username), []byte(password))
	if err != nil {
		return nil, err
	}

	basicAuth := &BasicAuth{
		Name: s.Name,

		Username: s.Username,
		Password: password,
		auth:     auth,
	}

	return basicAuth, nil
}

// SecretData computes the data map which can be used in a Kubernetes secret.
func (b *BasicAuth) SecretData() map[string][]byte {
	data := make(map[string][]byte, 3)

	data[DataKeyUserName] = []byte(b.Username)
	data[DataKeyPassword] = []byte(b.Password)
	data[DataKeyAuth] = b.auth

	return data
}
