// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenermetricscollector

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	openTelemetryCollectorName = "gardener-metrics"

	metricsPort = 2723

	internalMetricsPort = 8888

	managedResourceNameRuntime = "gardener-metrics-collector-runtime"

	managedResourceNameVirtual = "gardener-metrics-collector-virtual"

	roleName = "gardener-metrics"
)

var timeoutWaitForManagedResource = 5 * time.Minute

// Values contains the configuration values for the Gardener metrics collector component.
type Values struct {
	Image string
}

// New creates a new instance of the Gardener metrics collector component.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) component.DeployWaiter {
	return &gardenerMetricsCollector{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type gardenerMetricsCollector struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

func (g *gardenerMetricsCollector) Deploy(ctx context.Context) error {
	var (
		runtimeRegistry           = managedresources.NewRegistry(operatorclient.RuntimeScheme, operatorclient.RuntimeCodec, operatorclient.RuntimeSerializer)
		virtualGardenAccessSecret = g.newVirtualGardenAccessSecret()
		conditionTypeLabels       = map[string]string{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy}
	)

	if err := virtualGardenAccessSecret.Reconcile(ctx, g.client); err != nil {
		return err
	}

	secretGenericTokenKubeconfig, found := g.secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameGenericTokenKubeconfig)
	}

	runtimeResources, err := runtimeRegistry.AddAllAndSerialize(
		g.serviceAccount(),
		g.openTelemetryCollector(secretGenericTokenKubeconfig.Name, virtualGardenAccessSecret.Secret.Name),
		g.serviceMonitor(),
		g.internalMetricsServiceMonitor(),
		g.verticalPodAutoscaler(),
	)
	if err != nil {
		return err
	}

	if err := managedresources.CreateForSeedWithLabels(ctx, g.client, g.namespace, managedResourceNameRuntime, false, conditionTypeLabels, runtimeResources); err != nil {
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

	return managedresources.CreateForShootWithLabels(ctx, g.client, g.namespace, managedResourceNameVirtual, managedresources.LabelValueGardener, false, conditionTypeLabels, virtualResources)
}

func (g *gardenerMetricsCollector) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForShoot(ctx, g.client, g.namespace, managedResourceNameVirtual); err != nil {
		return err
	}

	if err := managedresources.DeleteForSeed(ctx, g.client, g.namespace, managedResourceNameRuntime); err != nil {
		return err
	}

	return kubernetesutils.DeleteObjects(ctx, g.client, g.newVirtualGardenAccessSecret().Secret)
}

func (g *gardenerMetricsCollector) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResource)
	defer cancel()

	return flow.Parallel(
		func(ctx context.Context) error {
			return managedresources.WaitUntilHealthy(ctx, g.client, g.namespace, managedResourceNameRuntime)
		},
		func(ctx context.Context) error {
			return managedresources.WaitUntilHealthy(ctx, g.client, g.namespace, managedResourceNameVirtual)
		},
	)(timeoutCtx)
}

func (g *gardenerMetricsCollector) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeoutWaitForManagedResource)
	defer cancel()

	return flow.Parallel(
		func(ctx context.Context) error {
			return managedresources.WaitUntilDeleted(ctx, g.client, g.namespace, managedResourceNameRuntime)
		},
		func(ctx context.Context) error {
			return managedresources.WaitUntilDeleted(ctx, g.client, g.namespace, managedResourceNameVirtual)
		},
	)(timeoutCtx)
}

func (g *gardenerMetricsCollector) newVirtualGardenAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret(openTelemetryCollectorName, g.namespace)
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelGardener,
		v1beta1constants.LabelRole: roleName,
	}
}
