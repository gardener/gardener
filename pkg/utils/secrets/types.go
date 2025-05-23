// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets

// ConfigInterface define functions needed for generating a specific secret.
type ConfigInterface interface {
	// GetName returns the name of the configuration.
	GetName() string
	// Generate generates a secret interface
	Generate() (DataInterface, error)
}

// DataInterface defines functions needed for defining the data map of a Kubernetes secret.
type DataInterface interface {
	// SecretData computes the data map which can be used in a Kubernetes secret.
	SecretData() map[string][]byte
}
