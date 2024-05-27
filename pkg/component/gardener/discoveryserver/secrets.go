// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package discoveryserver

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

const serviceAccountIssuerConfigSecretName = v1beta1constants.GardenRoleShootServiceAccountIssuer

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
			"hostname": g.hostname(),
		},
	}
}
