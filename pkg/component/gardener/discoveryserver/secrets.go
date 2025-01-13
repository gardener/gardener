// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package discoveryserver

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/keyutil"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/workloadidentity"
)

const (
	serviceAccountIssuerConfigSecretName = v1beta1constants.GardenRoleShootServiceAccountIssuer
	openIDConfigDataKey                  = "openid-configuration.json"
	jwksDataKey                          = "jwks.json"
)

func (g *gardenerDiscoveryServer) newVirtualGardenAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret(deploymentName, g.namespace)
}

func (g *gardenerDiscoveryServer) newServiceAccountIssuerConfigSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountIssuerConfigSecretName,
			Namespace: v1beta1constants.GardenNamespace,
			Labels: utils.MergeStringMaps(labels(), map[string]string{
				v1beta1constants.GardenRole: v1beta1constants.GardenRoleShootServiceAccountIssuer,
			}),
		},
		StringData: map[string]string{
			"hostname": g.values.Domain,
		},
	}
}

func (g *gardenerDiscoveryServer) workloadIdentitySecret() (*corev1.Secret, error) {
	bundleSecret, ok := g.secretsManager.Get(operatorv1alpha1.SecretNameWorkloadIdentityKey, manager.Bundle)
	if !ok {
		return nil, fmt.Errorf("failed to get secret %q", operatorv1alpha1.SecretNameWorkloadIdentityKey)
	}

	bundle, ok := bundleSecret.Data[secretsutils.DataKeyPrivateKeyBundle]
	if !ok {
		return nil, fmt.Errorf("secret %q is missing data key %q", bundleSecret.Name, secretsutils.DataKeyPrivateKeyBundle)
	}

	keys, err := keyutil.ParsePublicKeysPEM(bundle)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public keys from the bundle: %w", err)
	}

	openIDConfig, err := workloadidentity.OpenIDConfig(g.values.WorkloadIdentityTokenIssuer, keys...)
	if err != nil {
		return nil, fmt.Errorf("failed to construct openid configuration: %w", err)
	}

	jwks, err := workloadidentity.JWKS(keys...)
	if err != nil {
		return nil, fmt.Errorf("failed to construct jwks: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName + "-garden-workload-identity",
			Namespace: g.namespace,
			Labels:    labels(),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			openIDConfigDataKey: openIDConfig,
			jwksDataKey:         jwks,
		},
	}

	utilruntime.Must(kubernetes.MakeUnique(secret))
	return secret, nil
}
