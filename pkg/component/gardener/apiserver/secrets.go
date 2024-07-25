// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/component/apiserver"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	secretAuditWebhookKubeconfigNamePrefix = "gardener-apiserver-audit-webhook-kubeconfig"
	secretAdmissionKubeconfigsNamePrefix   = "gardener-apiserver-admission-kubeconfigs"
	secretWorkloadIdentityNamePrefix       = "gardener-apiserver-workload-identity-signing-key"
)

func (g *gardenerAPIServer) emptySecret(name string) *corev1.Secret {
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: g.namespace}}
}

func (g *gardenerAPIServer) newVirtualGardenAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret(DeploymentName, g.namespace)
}

func (g *gardenerAPIServer) reconcileSecretETCDEncryptionConfiguration(ctx context.Context, secret *corev1.Secret) error {
	return apiserver.ReconcileSecretETCDEncryptionConfiguration(
		ctx,
		g.client,
		g.secretsManager,
		g.values.ETCDEncryption,
		secret,
		v1beta1constants.SecretNameGardenerETCDEncryptionKey,
		v1beta1constants.SecretNamePrefixGardenerETCDEncryptionConfiguration,
	)
}

func (g *gardenerAPIServer) reconcileSecretServer(ctx context.Context) (*corev1.Secret, error) {
	return g.secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
		Name:                        secretNameServerCert,
		CommonName:                  serviceName,
		DNSNames:                    append(kubernetesutils.DNSNamesForService(DeploymentName, g.namespace), kubernetesutils.DNSNamesForService(DeploymentName, metav1.NamespaceSystem)...),
		CertType:                    secretsutils.ServerCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(operatorv1alpha1.SecretNameCAGardener), secretsmanager.Rotate(secretsmanager.InPlace))
}

func (g *gardenerAPIServer) reconcileWorkloadIdentityKey(ctx context.Context) (*corev1.Secret, error) {
	options := []secretsmanager.GenerateOption{
		secretsmanager.Persist(),
		secretsmanager.Rotate(secretsmanager.KeepOld), // TODO(vpnachev): Implement rotation triggers in Garden resource. Revise rotation strategy, if needed.
	}

	return g.secretsManager.Generate(ctx, &secretsutils.RSASecretConfig{
		Name: secretWorkloadIdentityNamePrefix,
		Bits: 4096,
	}, options...)
}
