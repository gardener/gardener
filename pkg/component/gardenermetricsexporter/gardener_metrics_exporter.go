// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardenermetricsexporter

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	deploymentName = "gardener-metrics-exporter"
	serviceName    = deploymentName

	probePort = 2718
	// ManagedResourceNameRuntime is the name of the ManagedResource for the runtime resources.
	ManagedResourceNameRuntime = "gardener-metrics-exporter-runtime"
	// ManagedResourceNameVirtual is the name of the ManagedResource for the virtual resources.
	ManagedResourceNameVirtual = "gardener-metrics-exporter-virtual"

	roleName = "metrics-exporter"
)

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy or
// deleted.
var TimeoutWaitForManagedResource = 5 * time.Minute

// Values is a set of configuration values for the gardener-metrics-exporter component.
type Values struct {
	// Image is the container image used for gardener-metrics-exporter.
	Image string
}

// New creates a new instance of DeployWaiter for gardener-metrics-exporter.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) component.DeployWaiter {
	return &gardenerMetricsExporter{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type gardenerMetricsExporter struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

func (g *gardenerMetricsExporter) Deploy(ctx context.Context) error {
	var (
		runtimeRegistry           = managedresources.NewRegistry(operatorclient.RuntimeScheme, operatorclient.RuntimeCodec, operatorclient.RuntimeSerializer)
		virtualGardenAccessSecret = g.newVirtualGardenAccessSecret()
	)

	if err := virtualGardenAccessSecret.Reconcile(ctx, g.client); err != nil {
		return err
	}

	secretGenericTokenKubeconfig, found := g.secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameGenericTokenKubeconfig)
	}

	runtimeResources, err := runtimeRegistry.AddAllAndSerialize(
		g.deployment(secretGenericTokenKubeconfig.Name, virtualGardenAccessSecret.Secret.Name),
		g.service(),
	)
	if err != nil {
		return err
	}

	if err := managedresources.CreateForSeed(ctx, g.client, g.namespace, ManagedResourceNameRuntime, false, runtimeResources); err != nil {
		return err
	}

	virtualRegistry := managedresources.NewRegistry(operatorclient.VirtualScheme, operatorclient.VirtualCodec, operatorclient.VirtualSerializer)

	virtualResources, err := virtualRegistry.AddAllAndSerialize(
		g.clusterRole(),
		g.clusterRoleBinding(virtualGardenAccessSecret.ServiceAccountName),
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, g.client, g.namespace, ManagedResourceNameVirtual, managedresources.LabelValueGardener, false, virtualResources)
}

func (g *gardenerMetricsExporter) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForShoot(ctx, g.client, g.namespace, ManagedResourceNameVirtual); err != nil {
		return err
	}

	if err := managedresources.DeleteForSeed(ctx, g.client, g.namespace, ManagedResourceNameRuntime); err != nil {
		return err
	}

	return kubernetesutils.DeleteObjects(ctx, g.client, g.newVirtualGardenAccessSecret().Secret)
}

func (g *gardenerMetricsExporter) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return flow.Parallel(
		func(ctx context.Context) error {
			return managedresources.WaitUntilHealthy(ctx, g.client, g.namespace, ManagedResourceNameRuntime)
		},
		func(ctx context.Context) error {
			return managedresources.WaitUntilHealthy(ctx, g.client, g.namespace, ManagedResourceNameVirtual)
		},
	)(timeoutCtx)
}

func (g *gardenerMetricsExporter) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return flow.Parallel(
		func(ctx context.Context) error {
			return managedresources.WaitUntilDeleted(ctx, g.client, g.namespace, ManagedResourceNameRuntime)
		},
		func(ctx context.Context) error {
			return managedresources.WaitUntilDeleted(ctx, g.client, g.namespace, ManagedResourceNameVirtual)
		},
	)(timeoutCtx)
}

// GetLabels returns the labels for the gardener-metrics-exporter.
func GetLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelGardener,
		v1beta1constants.LabelRole: roleName,
	}
}
