// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpntlsauth

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	// SecretNameTLSAuth is the name of seed server tlsauth Secret.
	SecretNameTLSAuth = "vpn-seed-server-tlsauth" // #nosec G101 -- No credential.
)

// VPNTLSAuthConfigFromSecret is a configuration for a VPN TLS auth secret with the tlsauth key itself as part
// of the configuration.
type VPNTLSAuthConfigFromSecret struct {
	Name string
	Data map[string][]byte
}

var _ secretsutils.ConfigInterface = &VPNTLSAuthConfigFromSecret{}
var _ secretsutils.DataInterface = &VPNTLSAuthConfigFromSecret{}

// GetName returns the name of the secret.
func (s *VPNTLSAuthConfigFromSecret) GetName() string {
	return s.Name
}

// Generate implements ConfigInterface.
func (s *VPNTLSAuthConfigFromSecret) Generate() (secretsutils.DataInterface, error) {
	return s, nil
}

// SecretData computes the data map which can be used in a Kubernetes secret.
func (s *VPNTLSAuthConfigFromSecret) SecretData() map[string][]byte {
	return s.Data
}

// GenerateSecret generates a VPN TLS auth secret using the provided secrets manager.
// It is used for two-staged generation of tlsauth secret to include the tlsauth key in the secret name hash.
func GenerateSecret(ctx context.Context, secretsManager secretsmanager.Interface) (*corev1.Secret, error) {
	// generate a secret with the tlsauth key
	secretTLSAuthIntermediate, err := secretsManager.Generate(ctx, &secretsutils.VPNTLSAuthConfig{
		Name: SecretNameTLSAuth,
	}, secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return nil, err
	}

	// use the secret to get a secret with same data but including the tlsauth key itself in name hash
	secretTLSAuth, err := secretsManager.Generate(ctx, &VPNTLSAuthConfigFromSecret{
		Name: secretTLSAuthIntermediate.Name,
		Data: secretTLSAuthIntermediate.Data,
	}, secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return nil, err
	}
	return secretTLSAuth, nil
}
