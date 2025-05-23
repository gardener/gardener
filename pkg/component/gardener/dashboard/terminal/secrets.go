// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package terminal

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

func (t *terminal) reconcileSecretServerCert(ctx context.Context) (*corev1.Secret, error) {
	return t.secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
		Name:                        name,
		CommonName:                  name,
		DNSNames:                    kubernetesutils.DNSNamesForService(name, t.namespace),
		CertType:                    secretsutils.ServerCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(operatorv1alpha1.SecretNameCARuntime, secretsmanager.UseCurrentCA), secretsmanager.Rotate(secretsmanager.InPlace))
}

func (t *terminal) newVirtualGardenAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret(name, t.namespace)
}
