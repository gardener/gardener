// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets

import "github.com/gardener/gardener/pkg/utils/infodata"

// ConfigInterface define functions needed for generating a specific secret.
type ConfigInterface interface {
	// GetName returns the name of the configuration.
	GetName() string
	// Generate generates a secret interface
	Generate() (DataInterface, error)
	// GenerateInfoData generates only the InfoData (metadata) which can later be used to generate a secret.
	GenerateInfoData() (infodata.InfoData, error)
	// GenerateFromInfoData combines the configuration and the provided InfoData (metadata) and generates a secret.
	GenerateFromInfoData(infoData infodata.InfoData) (DataInterface, error)
}

// DataInterface defines functions needed for defining the data map of a Kubernetes secret.
type DataInterface interface {
	// SecretData computes the data map which can be used in a Kubernetes secret.
	SecretData() map[string][]byte
}
