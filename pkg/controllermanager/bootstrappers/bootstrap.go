// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/client-go/discovery"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/gardener/gardener/pkg/utils/version"
)

// Bootstrapper is a runnable for bootstrapping the garden cluster.
type Bootstrapper struct {
	Log        logr.Logger
	RESTConfig *rest.Config
}

// Start runs as soon as the manager got leader.
func (b *Bootstrapper) Start(parentCtx context.Context) error {
	// Other controllers depend on garden cluster bootstrapping.
	// Hence, if we can't bootstrap the garden cluster in a short timeout, terminate and try again after restart.
	ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer cancel()

	kubernetesClient, err := kubernetesclientset.NewForConfig(b.RESTConfig)
	if err != nil {
		return fmt.Errorf("failed creating kubernetes client: %w", err)
	}

	if err := bootstrapCluster(ctx, kubernetesClient.Discovery()); err != nil {
		return fmt.Errorf("failed bootstrapping garden cluster: %w", err)
	}

	b.Log.Info("Successfully bootstrapped Garden cluster")
	return nil
}

func bootstrapCluster(ctx context.Context, discoveryClient discovery.DiscoveryInterface) error {
	const minKubernetesVersion = "1.32"

	serverVersion, err := discoveryClient.ServerVersion()
	if err != nil {
		return fmt.Errorf("failed discovering garden cluster kubernetes version: %w", err)
	}

	gardenVersionOK, err := version.CompareVersions(serverVersion.GitVersion, ">=", minKubernetesVersion)
	if err != nil {
		return err
	}
	if !gardenVersionOK {
		return fmt.Errorf("the Kubernetes version of the Garden cluster must be at least %s", minKubernetesVersion)
	}

	return nil
}
