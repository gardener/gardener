// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controller/gardenletdeployer"
)

// IsGardenletDeployed indicates whether gardenlet is running in the shoot cluster.
// It checks for the existence of the gardenlet deployment and its kubeconfig secret in the control plane namespace.
func IsGardenletDeployed(ctx context.Context, c client.Client, namespace string) (bool, error) {
	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: v1beta1constants.DeploymentNameGardenlet}, &appsv1.Deployment{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("failed checking if gardenlet deployment already exists: %w", err)
		}
		return false, nil
	}

	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: gardenletdeployer.GardenletDefaultKubeconfigSecretName}, &corev1.Secret{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("failed checking if gardenlet's kubeconfig secret already exists: %w", err)
		}
		return false, nil
	}

	return true, nil
}
