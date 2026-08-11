// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenadm/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

// DefaultKubeconfig sets the given kubeconfig pointer to the value of the KUBECONFIG environment variable, or to the
// default kubeconfig path in the user's home directory if KUBECONFIG is not set.
func DefaultKubeconfig(kubeconfig *string) error {
	if kubeconfig == nil {
		return fmt.Errorf("kubeconfig pointer must not be nil")
	}

	if ptr.Deref(kubeconfig, "") != "" {
		return nil
	}

	if kubeconfigEnv := os.Getenv("KUBECONFIG"); kubeconfigEnv != "" {
		*kubeconfig = os.Getenv("KUBECONFIG")
		return nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}
	*kubeconfig = filepath.Join(homeDir, ".kube", "config")

	return nil
}

// InitializeGardenadmWithTemporaryClientSet initializes a GardenadmBotanist with a temporary client set based on the provided bootstrap token.
func InitializeGardenadmWithTemporaryClientSet(
	ctx context.Context,
	log logr.Logger,
	controlPlaneAddress string,
	certificateAuthority []byte,
	bootstrapToken string,
) (
	*botanist.GardenadmBotanist,
	error,
) {
	b, err := botanist.NewGardenadmBotanistWithoutResources(log)
	if err != nil {
		return nil, fmt.Errorf("failed creating gardenadm botanist: %w", err)
	}

	bootstrapClientSet, err := NewClientSetFromBootstrapToken(controlPlaneAddress, certificateAuthority, bootstrapToken, kubernetes.SeedScheme)
	if err != nil {
		return nil, fmt.Errorf("failed creating a new bootstrap client set: %w", err)
	}
	version, err := b.DiscoverKubernetesVersion(bootstrapClientSet)
	if err != nil {
		return nil, fmt.Errorf("failed discovering Kubernetes version of cluster: %w", err)
	}
	b.Shoot = &shootpkg.Shoot{KubernetesVersion: version, ControlPlaneNamespace: metav1.NamespaceSystem}
	b.Shoot.SetInfo(nil)

	b.Logger.Info("Retrieving short-lived shoot cluster kubeconfig via token")
	b.ShootClientSet, err = InitializeTemporaryClientSet(ctx, b, bootstrapClientSet)
	if err != nil {
		return nil, fmt.Errorf("failed retrieving short-lived kubeconfig: %w", err)
	}
	b.Logger.Info("Successfully retrieved short-lived kubeconfig")

	return b, nil
}
