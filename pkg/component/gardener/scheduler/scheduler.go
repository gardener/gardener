// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	// DeploymentName is the name of the deployment.
	DeploymentName = "gardener-scheduler"

	probePort   = 10251
	metricsPort = 19251

	// ManagedResourceNameRuntime is the name of the ManagedResource for the runtime resources.
	ManagedResourceNameRuntime = "gardener-scheduler-runtime"
	// ManagedResourceNameVirtual is the name of the ManagedResource for the virtual resources.
	ManagedResourceNameVirtual = "gardener-scheduler-virtual"
)

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy or
// deleted.
var TimeoutWaitForManagedResource = 5 * time.Minute

// Values contains configuration values for the gardener-scheduler resources.
type Values struct {
	// Image defines the container image of gardener-scheduler.
	Image string
	// LogLevel is the level/severity for the logs. Must be one of [info,debug,error].
	LogLevel string
	// FeatureGates is the set of feature gates.
	FeatureGates map[string]bool
}

// New creates a new instance of DeployWaiter for the gardener-scheduler.
func New(client client.Client, namespace string, secretsManager secretsmanager.Interface, values Values) component.DeployWaiter {
	return &gardenerScheduler{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type gardenerScheduler struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

func (g *gardenerScheduler) Deploy(ctx context.Context) error {
	var (
		runtimeRegistry           = managedresources.NewRegistry(operatorclient.RuntimeScheme, operatorclient.RuntimeCodec, operatorclient.RuntimeSerializer)
		virtualGardenAccessSecret = g.newVirtualGardenAccessSecret()
		managedResourceLabels     = map[string]string{v1beta1constants.LabelCareConditionType: string(operatorv1alpha1.VirtualComponentsHealthy)}
	)

	if err := virtualGardenAccessSecret.Reconcile(ctx, g.client); err != nil {
		return err
	}

	schedulerConfigConfigMap, err := g.configMapSchedulerConfig()
	if err != nil {
		return err
	}

	secretGenericTokenKubeconfig, found := g.secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameGenericTokenKubeconfig)
	}

	runtimeResources, err := runtimeRegistry.AddAllAndSerialize(
		schedulerConfigConfigMap,
		g.podDisruptionBudget(),
		g.service(),
		g.verticalPodAutoscaler(),
		g.deployment(secretGenericTokenKubeconfig.Name, virtualGardenAccessSecret.Secret.Name, schedulerConfigConfigMap.Name),
		g.serviceMonitor(),
	)
	if err != nil {
		return err
	}

	if err := managedresources.CreateForSeedWithLabels(ctx, g.client, g.namespace, ManagedResourceNameRuntime, false, managedResourceLabels, runtimeResources); err != nil {
		return err
	}

	var (
		virtualRegistry = managedresources.NewRegistry(operatorclient.VirtualScheme, operatorclient.VirtualCodec, operatorclient.VirtualSerializer)
	)

	virtualResources, err := virtualRegistry.AddAllAndSerialize(
		g.clusterRole(),
		g.clusterRoleBinding(virtualGardenAccessSecret.ServiceAccountName),
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForShootWithLabels(ctx, g.client, g.namespace, ManagedResourceNameVirtual, managedresources.LabelValueGardener, false, managedResourceLabels, virtualResources)
}

func (g *gardenerScheduler) Wait(ctx context.Context) error {
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

func (g *gardenerScheduler) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForShoot(ctx, g.client, g.namespace, ManagedResourceNameVirtual); err != nil {
		return err
	}

	if err := managedresources.DeleteForSeed(ctx, g.client, g.namespace, ManagedResourceNameRuntime); err != nil {
		return err
	}

	return kubernetesutils.DeleteObjects(ctx, g.client, g.newVirtualGardenAccessSecret().Secret)
}

func (g *gardenerScheduler) WaitCleanup(ctx context.Context) error {
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

// GetLabels returns the labels for the gardener-scheduler.
func GetLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelGardener,
		v1beta1constants.LabelRole: v1beta1constants.LabelScheduler,
	}
}
