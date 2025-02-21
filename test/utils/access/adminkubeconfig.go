// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package access

import (
	"context"
	"fmt"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	authenticationv1alpha1 "github.com/gardener/gardener/pkg/apis/authentication/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// CreateShootClientFromAdminKubeconfig requests an admin kubeconfig and creates a shoot client.
func CreateShootClientFromAdminKubeconfig(ctx context.Context, gardenClient kubernetes.Interface, shoot *gardencorev1beta1.Shoot) (kubernetes.Interface, error) {
	return createShootClientFromDynamicKubeconfig(ctx, gardenClient, shoot, RequestAdminKubeconfigForShoot)
}

// CreateShootClientFromViewerKubeconfig requests a viewer kubeconfig and creates a shoot client.
func CreateShootClientFromViewerKubeconfig(ctx context.Context, gardenClient kubernetes.Interface, shoot *gardencorev1beta1.Shoot) (kubernetes.Interface, error) {
	return createShootClientFromDynamicKubeconfig(ctx, gardenClient, shoot, RequestViewerKubeconfigForShoot)
}

func createShootClientFromDynamicKubeconfig(
	ctx context.Context,
	gardenClient kubernetes.Interface,
	shoot *gardencorev1beta1.Shoot,
	requestFn func(context.Context, kubernetes.Interface, *gardencorev1beta1.Shoot, *int64) ([]byte, error),
) (
	kubernetes.Interface,
	error,
) {
	kubeconfig, err := requestFn(ctx, gardenClient, shoot, ptr.To[int64](7200))
	if err != nil {
		return nil, err
	}

	return kubernetes.NewClientFromBytes(
		kubeconfig,
		kubernetes.WithClientOptions(client.Options{Scheme: kubernetes.ShootScheme}),
		kubernetes.WithDisabledCachedClient(),
	)
}

// RequestAdminKubeconfigForShoot requests an admin kubeconfig for the given shoot.
func RequestAdminKubeconfigForShoot(ctx context.Context, gardenClient kubernetes.Interface, shoot *gardencorev1beta1.Shoot, expirationSeconds *int64) ([]byte, error) {
	adminKubeconfigRequest := &authenticationv1alpha1.AdminKubeconfigRequest{
		Spec: authenticationv1alpha1.AdminKubeconfigRequestSpec{
			ExpirationSeconds: expirationSeconds,
		},
	}
	if err := gardenClient.Client().SubResource("adminkubeconfig").Create(ctx, shoot, adminKubeconfigRequest); err != nil {
		return nil, fmt.Errorf("failed to create admin kubeconfig request for shoot %s: %w", client.ObjectKeyFromObject(shoot), err)
	}

	return adminKubeconfigRequest.Status.Kubeconfig, nil
}

// RequestViewerKubeconfigForShoot requests a viewer kubeconfig for the given shoot.
func RequestViewerKubeconfigForShoot(ctx context.Context, gardenClient kubernetes.Interface, shoot *gardencorev1beta1.Shoot, expirationSeconds *int64) ([]byte, error) {
	viewerKubeconfigRequest := &authenticationv1alpha1.ViewerKubeconfigRequest{
		Spec: authenticationv1alpha1.ViewerKubeconfigRequestSpec{
			ExpirationSeconds: expirationSeconds,
		},
	}
	if err := gardenClient.Client().SubResource("viewerkubeconfig").Create(ctx, shoot, viewerKubeconfigRequest); err != nil {
		return nil, err
	}

	return viewerKubeconfigRequest.Status.Kubeconfig, nil
}
