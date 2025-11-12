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
// resources, e.g., Services, NetworkPolicies, machine Pods, etc. If the provider secret contains a kubeconfig,
// a client for that kubeconfig is created. Otherwise, the given client for the runtime cluster is returned.
// See https://github.com/gardener/gardener/blob/master/docs/extensions/provider-local.md#credentials.
func GetProviderClient(ctx context.Context, log logr.Logger, runtimeClient client.Client, secretRef corev1.SecretReference) (client.Client, error) {
	providerSecret, err := kubernetesutils.GetSecretByReference(ctx, runtimeClient, &secretRef)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve provider secret: %w", err)
	}

	if len(providerSecret.Data[kubernetes.KubeConfig]) == 0 {
		log.Info("Using in-cluster config for provider client as no kubeconfig is specified in the provider secret")
		return runtimeClient, nil
	}

	clientSet, err := kubernetes.NewClientFromBytes(providerSecret.Data[kubernetes.KubeConfig],
		kubernetes.WithClientOptions(client.Options{Scheme: kubernetes.SeedScheme}),
		kubernetes.WithDisabledCachedClient(),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create client from provider secret: %w", err)
	}

	log.Info("Using kubeconfig from provider secret for provider client")
	return clientSet.Client(), nil
}
