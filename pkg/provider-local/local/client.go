// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// GetProviderClient returns a Kubernetes client for the cluster in which provider-local should manage infrastructure
// resources, i.e. machine Pods. It is constructed from the provider secret.
// See https://github.com/gardener/gardener/blob/master/docs/extensions/provider-local.md#credentials.
func GetProviderClient(ctx context.Context, _ logr.Logger, runtimeClient client.Client, secretRef corev1.SecretReference) (client.Client, error) {
	providerSecret, err := kubernetesutils.GetSecretByReference(ctx, runtimeClient, &secretRef)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve provider secret: %w", err)
	}

	if len(providerSecret.Data[kubernetes.KubeConfig]) == 0 {
		// should be unreachable, because the secret is validated.
		return nil, fmt.Errorf("no kubeconfig found in provider secret")
	}

	clientSet, err := kubernetes.NewClientFromBytes(providerSecret.Data[kubernetes.KubeConfig],
		kubernetes.WithClientOptions(client.Options{Scheme: kubernetes.SeedScheme}),
		kubernetes.WithDisabledCachedClient(),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create client from provider secret: %w", err)
	}

	return clientSet.Client(), nil
}
