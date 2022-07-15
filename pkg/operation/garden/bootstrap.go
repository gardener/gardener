// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package garden

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/discovery"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	"github.com/gardener/gardener/pkg/utils/version"
)

// Bootstrapper is a runnable for bootstrapping the garden cluster.
type Bootstrapper struct {
	Log        logr.Logger
	Client     client.Client
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

	secretsManager, err := secretsmanager.New(ctx, b.Log.WithName("secretsmanager"), clock.RealClock{}, b.Client, v1beta1constants.GardenNamespace, v1beta1constants.SecretManagerIdentityControllerManager, secretsmanager.Config{})
	if err != nil {
		return fmt.Errorf("failed creating new secrets manager: %w", err)
	}

	if err := bootstrapCluster(ctx, b.Client, kubernetesClient.Discovery(), secretsManager); err != nil {
		return fmt.Errorf("failed bootstrapping garden cluster: %w", err)
	}

	if err := secretsManager.Cleanup(ctx); err != nil {
		return fmt.Errorf("failed cleaning up no longer required secrets: %w", err)
	}

	b.Log.Info("Successfully bootstrapped Garden cluster")
	return nil
}

func bootstrapCluster(ctx context.Context, gardenClient client.Client, discoveryClient discovery.DiscoveryInterface, secretsManager secretsmanager.Interface) error {
	const minKubernetesVersion = "1.20"

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

	secretList := &corev1.SecretList{}
	if err := gardenClient.List(
		ctx,
		secretList,
		client.InNamespace(v1beta1constants.GardenNamespace),
		client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleGlobalMonitoring},
	); err != nil {
		return err
	}

	mustGenerateMonitoringSecret := true
	for _, s := range secretList.Items {
		managedBySecretsManager := s.Labels[secretsmanager.LabelKeyManagedBy] == secretsmanager.LabelValueSecretsManager &&
			s.Labels[secretsmanager.LabelKeyManagerIdentity] == v1beta1constants.SecretManagerIdentityControllerManager

		if !managedBySecretsManager {
			// found a custom monitoring secret managed by a human operator
			// keep it and don't take over responsibility for the monitoring secret
			mustGenerateMonitoringSecret = false
			break
		}
	}

	// we don't want to override custom monitoring secret managed by a human operator
	// only take over responsibility over monitoring secret if we find the legacy secret created by GCM or a new one managed by SecretsManager
	if mustGenerateMonitoringSecret {
		if _, err = generateGlobalMonitoringSecret(ctx, gardenClient, secretsManager); err != nil {
			return err
		}
	}

	return nil
}
