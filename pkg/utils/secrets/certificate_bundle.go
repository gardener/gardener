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
	"errors"

	"github.com/gardener/gardener/pkg/utils/infodata"
)

// DataKeyCertificateBundle is the key in the data map for the certificate bundle.
const DataKeyCertificateBundle = "bundle.crt"

// CertificateBundleSecretConfig is configuration for certificate bundles.
type CertificateBundleSecretConfig struct {
	Name            string
	CertificatePEMs [][]byte
}

// CertificateBundle contains the name and the generated certificate bundle.
type CertificateBundle struct {
	Name   string
	Bundle []byte
}

// GetName returns the name of the secret.
func (s *CertificateBundleSecretConfig) GetName() string {
	return s.Name
}

// Generate implements ConfigInterface.
func (s *CertificateBundleSecretConfig) Generate() (DataInterface, error) {
	return &CertificateBundle{
		Name:   s.Name,
		Bundle: s.generateBundle(),
	}, nil
}

func (s *CertificateBundleSecretConfig) generateBundle() []byte {
	var bundle []byte
	for _, pem := range s.CertificatePEMs {
		bundle = append(bundle, pem...)
	}
	return bundle
}

// GenerateInfoData implements ConfigInterface.
func (s *CertificateBundleSecretConfig) GenerateInfoData() (infodata.InfoData, error) {
	return nil, errors.New("not implemented")
}

// GenerateFromInfoData implements ConfigInterface.
func (s *CertificateBundleSecretConfig) GenerateFromInfoData(_ infodata.InfoData) (DataInterface, error) {
	return nil, errors.New("not implemented")
}

// LoadFromSecretData implements infodata.Loader.
func (s *CertificateBundleSecretConfig) LoadFromSecretData(_ map[string][]byte) (infodata.InfoData, error) {
	return nil, errors.New("not implemented")
}

// SecretData computes the data map which can be used in a Kubernetes secret.
func (v *CertificateBundle) SecretData() map[string][]byte {
	return map[string][]byte{DataKeyCertificateBundle: v.Bundle}
}
