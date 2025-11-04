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
// resources, e.g., Services, NetworkPolicies, machine Pods, etc. If the cloudprovider secret contains a kubeconfig,
// a client for that kubeconfig is created. Otherwise, the given seedClient is returned.
// See https://github.com/gardener/gardener/blob/master/docs/extensions/provider-local.md#credentials.
func GetProviderClient(ctx context.Context, log logr.Logger, seedClient client.Client, secretRef corev1.SecretReference) (client.Client, error) {
	cloudProviderSecret, err := kubernetesutils.GetSecretByReference(ctx, seedClient, &secretRef)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve cloudprovider secret: %w", err)
	}

	if len(cloudProviderSecret.Data[kubernetes.KubeConfig]) == 0 {
		log.Info("Using in-cluster config for provider client as no kubeconfig is specified in the cloudprovider secret")
		return seedClient, nil
	}

	clientSet, err := kubernetes.NewClientFromBytes(cloudProviderSecret.Data[kubernetes.KubeConfig],
		kubernetes.WithClientOptions(client.Options{Scheme: kubernetes.SeedScheme}),
		kubernetes.WithDisabledCachedClient(),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create client from cloudprovider secret: %w", err)
	}

	log.Info("Using kubeconfig from cloudprovider secret for provider client")
	return clientSet.Client(), nil
}
