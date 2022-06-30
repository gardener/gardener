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

package kubestatemetrics

import (
	"context"
	"fmt"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ManagedResourceName is the name of the managed resource for seeds.
	ManagedResourceName      = "kube-state-metrics"
	shootManagedResourceName = "shoot-core-" + ManagedResourceName

	containerName = "kube-state-metrics"

	labelKeyComponent   = "component"
	labelKeyType        = "type"
	labelValueComponent = "kube-state-metrics"

	port = 8080
)

// Interface contains functions for a kube-state-metrics deployer.
type Interface interface {
	component.DeployWaiter
	component.MonitoringComponent
}

// New creates a new instance of DeployWaiter for the kube-state-metrics.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) Interface {
	k := &kubeStateMetrics{
		client:         client,
		secretsManager: secretsManager,
		namespace:      namespace,
		values:         values,
	}

	if values.ClusterType == component.ClusterTypeSeed {
		k.registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
	} else {
		k.registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
	}

	return k
}

type kubeStateMetrics struct {
	client         client.Client
	secretsManager secretsmanager.Interface
	namespace      string
	values         Values
	registry       *managedresources.Registry
}

// Values is a set of configuration values for the kube-state-metrics.
type Values struct {
	// ClusterType specifies the type of the cluster to which kube-state-metrics is being deployed.
	// For seeds, all resources are being deployed as part of a ManagedResource.
	// For shoots, the kube-state-metrics runs in the shoot namespace in the seed as part of the control plane. Hence,
	// only the runtime resources (like Deployment, Service, etc.) are being deployed directly (with the client). All
	// other application-related resources (like RBAC roles, CRD, etc.) are deployed as part of a ManagedResource.
	ClusterType component.ClusterType
	// Image is the container image.
	Image string
	// Replicas is the number of replicas.
	Replicas int32
}

func (k *kubeStateMetrics) Deploy(ctx context.Context) error {
	var (
		genericTokenKubeconfigSecretName string
		shootAccessSecret                *gutil.ShootAccessSecret
	)

	if k.values.ClusterType == component.ClusterTypeShoot {
		genericTokenKubeconfigSecret, found := k.secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
		if !found {
			return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameGenericTokenKubeconfig)
		}
		genericTokenKubeconfigSecretName = genericTokenKubeconfigSecret.Name

		shootAccessSecret = k.newShootAccessSecret()
		if err := shootAccessSecret.Reconcile(ctx, k.client); err != nil {
			return err
		}
	}

	return component.DeployResourceConfigs(ctx, k.client, k.namespace, k.values.ClusterType, k.managedResourceName(), k.registry, k.getResourceConfigs(genericTokenKubeconfigSecretName, shootAccessSecret))
}

func (k *kubeStateMetrics) Destroy(ctx context.Context) error {
	if err := component.DestroyResourceConfigs(ctx, k.client, k.namespace, k.values.ClusterType, k.managedResourceName(), k.getResourceConfigs("", nil)); err != nil {
		return err
	}

	if k.values.ClusterType == component.ClusterTypeShoot {
		return client.IgnoreNotFound(k.client.Delete(ctx, k.newShootAccessSecret().Secret))
	}

	return nil
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (k *kubeStateMetrics) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, k.client, k.namespace, k.managedResourceName())
}

func (k *kubeStateMetrics) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, k.client, k.namespace, k.managedResourceName())
}

func (k *kubeStateMetrics) managedResourceName() string {
	if k.values.ClusterType == component.ClusterTypeSeed {
		return ManagedResourceName
	}
	return shootManagedResourceName
}
