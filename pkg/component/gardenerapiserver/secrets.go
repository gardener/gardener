// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardenerapiserver

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/apiserver"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	secretAuditWebhookKubeconfigNamePrefix = "gardener-apiserver-audit-webhook-kubeconfig"
)

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
		Name:                        secretNameServer,
		CommonName:                  DeploymentName,
		DNSNames:                    append(kubernetesutils.DNSNamesForService(DeploymentName, g.namespace), kubernetesutils.DNSNamesForService(DeploymentName, metav1.NamespaceSystem)...),
		CertType:                    secretsutils.ServerCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCAGardener), secretsmanager.Rotate(secretsmanager.InPlace))
}
