// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controller

import (
	"context"
	"fmt"

	gardencorev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

// GenerateVirtualGardenKubeconfig generates a kubeconfig for the specified service account token in the garden namespace in the virtual garden cluster
func (o *operation) GenerateVirtualGardenKubeconfig(ctx context.Context, serviceAccountName string) (*string, error) {
	tokenAPIServer, err := o.getServiceAccountToken(ctx, serviceAccountName)
	if err != nil {
		return nil, err
	}

	return o.generateKubeconfig(*tokenAPIServer, string(o.virtualGardenCA))
}

// getServiceAccountToken gets the service account token for the provided service account in the garden namespace
func (o *operation) getServiceAccountToken(ctx context.Context, serviceAccountName string) (*string, error) {
	secretName, err := o.getSecretNameFromServiceAccount(ctx, serviceAccountName)
	if err != nil {
		return nil, err
	}
	saSecret := &corev1.Secret{}
	if err := o.getGardenClient().Client().Get(ctx, kutil.Key(gardencorev1beta1constants.GardenNamespace, *secretName), saSecret); err != nil {
		return nil, fmt.Errorf("failed to retrieve service account secret %s/%s from the runtime cluster: %v", gardencorev1beta1constants.GardenNamespace, *secretName, err)
	}

	if len(saSecret.Data) == 0 {
		return nil, fmt.Errorf("service account secret %s/%s in the runtime cluster does not contain a JWT token: %v", gardencorev1beta1constants.GardenNamespace, *secretName, err)
	}

	token, ok := saSecret.Data["token"]
	if !ok {
		return nil, fmt.Errorf("service account secret %s/%s in the runtime cluster does not contain a JWT token: %v", gardencorev1beta1constants.GardenNamespace, *secretName, err)
	}

	return pointer.String(string(token)), nil
}

// getSecretNameFromServiceAccount gets the secret name for a service account
func (o *operation) getSecretNameFromServiceAccount(ctx context.Context, name string) (*string, error) {
	sa := &corev1.ServiceAccount{}
	if err := o.getGardenClient().Client().Get(ctx, kutil.Key(gardencorev1beta1constants.GardenNamespace, name), sa); err != nil {
		return nil, fmt.Errorf("failed to retrieve service account %s/%s from the runtime cluster: %v", gardencorev1beta1constants.GardenNamespace, name, err)
	}
	if len(sa.Secrets) == 0 {
		return nil, fmt.Errorf("no secret found for service account %s/%s in the runtime cluster", gardencorev1beta1constants.GardenNamespace, name)
	}
	return &sa.Secrets[0].Name, nil
}

// generateKubeconfig generates a kubeconfig based on the given token, cluster endpoint and CA certificate
func (o *operation) generateKubeconfig(token, caPem string) (*string, error) {
	secretConfig := &secrets.ControlPlaneSecretConfig{
		Token: &secrets.Token{
			Token: token,
		},
		KubeConfigRequests: []secrets.KubeConfigRequest{
			{
				ClusterName:   deploymentNameGardenerAPIServer,
				APIServerHost: *o.VirtualGardenClusterEndpoint,
			},
		},
	}

	certConfig := &secrets.Certificate{
		CA: &secrets.Certificate{
			CertificatePEM: []byte(caPem),
		},
	}

	apiServerKubeconfig, err := secrets.GenerateKubeconfig(secretConfig, certConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to generate the kubeconfig: %w", err)
	}

	return pointer.String(string(apiServerKubeconfig)), nil
}
