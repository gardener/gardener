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

const (
	// DataKeyCertificateBundle is the key in the data map for the certificate bundle.
	DataKeyCertificateBundle = "bundle.crt"
	// DataKeyPrivateKeyBundle is the key in the data map for the private key bundle.
	DataKeyPrivateKeyBundle = "bundle.key"
)

// CertificateBundleSecretConfig is configuration for certificate bundles.
type CertificateBundleSecretConfig struct {
	Name            string
	CertificatePEMs [][]byte
}

// GetName returns the name of the secret.
func (s *CertificateBundleSecretConfig) GetName() string {
	return s.Name
}

// Generate implements ConfigInterface.
func (s *CertificateBundleSecretConfig) Generate() (DataInterface, error) {
	return newBundle(s.Name, s.CertificatePEMs, DataKeyCertificateBundle)
}

// RSAPrivateKeyBundleSecretConfig is configuration for certificate bundles.
type RSAPrivateKeyBundleSecretConfig struct {
	Name           string
	PrivateKeyPEMs [][]byte
}

// GetName returns the name of the secret.
func (s *RSAPrivateKeyBundleSecretConfig) GetName() string {
	return s.Name
}

// Generate implements ConfigInterface.
func (s *RSAPrivateKeyBundleSecretConfig) Generate() (DataInterface, error) {
	return newBundle(s.Name, s.PrivateKeyPEMs, DataKeyPrivateKeyBundle)
}

func newBundle(name string, entries [][]byte, dataKeyName string) (DataInterface, error) {
	var bundle []byte
	for _, entry := range entries {
		bundle = append(bundle, entry...)
	}

	return &Bundle{
		Name:        name,
		Bundle:      bundle,
		DataKeyName: dataKeyName,
	}, nil
}

// Bundle contains the name and the generated certificate bundle.
type Bundle struct {
	Name        string
	Bundle      []byte
	DataKeyName string
}

// SecretData computes the data map which can be used in a Kubernetes secret.
func (b *Bundle) SecretData() map[string][]byte {
	return map[string][]byte{b.DataKeyName: b.Bundle}
}
