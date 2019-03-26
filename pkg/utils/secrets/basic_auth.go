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
	"fmt"
	"strings"

	"github.com/gardener/gardener/pkg/utils"
	"k8s.io/apiserver/pkg/authentication/user"
)

type formatType string

const (
	// BasicAuthFormatNormal indicates that the data map should be rendered the normal way (dedicated keys for
	// username and password.
	BasicAuthFormatNormal formatType = "normal"
	// BasicAuthFormatCSV indicates that the data map should be rendered in the CSV-format.
	BasicAuthFormatCSV formatType = "csv"

	// DataKeyCSV is the key in a secret data holding the CSV format of a secret.
	DataKeyCSV = "basic_auth.csv"
	// DataKeyUserName is the key in a secret data holding the username.
	DataKeyUserName = "username"
	// DataKeyPassword is the key in a secret data holding the password.
	DataKeyPassword = "password"
)

// BasicAuthSecretConfig contains the specification a to-be-generated basic authentication secret.
type BasicAuthSecretConfig struct {
	Name   string
	Format formatType

	Username       string
	PasswordLength int
}

// BasicAuth contains the username, the password and the format for serializing the basic authentication
// secret.
type BasicAuth struct {
	Name   string
	Format formatType

	Username string
	Password string
}

// GetName returns the name of the secret.
func (s *BasicAuthSecretConfig) GetName() string {
	return s.Name
}

// Generate implements ConfigInterface.
func (s *BasicAuthSecretConfig) Generate() (Interface, error) {
	return s.GenerateBasicAuth()
}

// GenerateBasicAuth computes a username/password keypair. It uses "admin" as username and generates a
// random password of length 32.
func (s *BasicAuthSecretConfig) GenerateBasicAuth() (*BasicAuth, error) {
	password, err := utils.GenerateRandomString(s.PasswordLength)
	if err != nil {
		return nil, err
	}

	return &BasicAuth{
		Name:   s.Name,
		Format: s.Format,

		Username: s.Username,
		Password: password,
	}, nil
}

// SecretData computes the data map which can be used in a Kubernetes secret.
func (b *BasicAuth) SecretData() map[string][]byte {
	data := map[string][]byte{}

	switch b.Format {
	case BasicAuthFormatNormal:
		data[DataKeyUserName] = []byte(b.Username)
		data[DataKeyPassword] = []byte(b.Password)
		fallthrough

	case BasicAuthFormatCSV:
		data[DataKeyCSV] = []byte(fmt.Sprintf("%s,%s,%s,%s", b.Password, b.Username, b.Username, user.SystemPrivilegedGroup))
	}

	return data
}

// LoadBasicAuthFromCSV loads the basic auth. username and the password from the given CSV-formatted <data>.
func LoadBasicAuthFromCSV(name string, data []byte) (*BasicAuth, error) {
	csv := strings.Split(string(data), ",")
	if len(csv) < 2 {
		return nil, fmt.Errorf("invalid CSV for loading basic auth data: %s", string(data))
	}

	return &BasicAuth{
		Name: name,

		Username: csv[1],
		Password: csv[0],
	}, nil
}
