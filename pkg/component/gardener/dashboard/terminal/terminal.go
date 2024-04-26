// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package terminal

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	managedResourceNameRuntime = "terminal-runtime"
	managedResourceNameVirtual = "terminal-virtual"

	name = "terminal-controller-manager"

	portNameAdmission = "webhook"
	portAdmission     = 9443
	portNameMetrics   = "metrics"
	portMetrics       = 8443
	portProbes        = 8081
)

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy or
// deleted.
var TimeoutWaitForManagedResource = 5 * time.Minute

// Values contains configuration values for the terminal-controller-manager resources.
type Values struct {
	// Image defines the container image of terminal-controller-manager.
	Image string
	// RuntimeVersion is the Kubernetes version of the runtime cluster.
	RuntimeVersion *semver.Version
	// TopologyAwareRoutingEnabled determines whether topology aware hints are intended.
	TopologyAwareRoutingEnabled bool
}

// New creates a new instance of DeployWaiter for the terminal-controller-manager.
func New(client client.Client, namespace string, secretsManager secretsmanager.Interface, values Values) component.DeployWaiter {
	return &terminal{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type terminal struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

func (t *terminal) Deploy(ctx context.Context) error {
	var (
		runtimeRegistry           = managedresources.NewRegistry(operatorclient.RuntimeScheme, operatorclient.RuntimeCodec, operatorclient.RuntimeSerializer)
		managedResourceLabels     = map[string]string{v1beta1constants.LabelCareConditionType: string(operatorv1alpha1.VirtualComponentsHealthy)}
		virtualGardenAccessSecret = t.newVirtualGardenAccessSecret()
	)

	if err := virtualGardenAccessSecret.Reconcile(ctx, t.client); err != nil {
		return err
	}

	secretGenericTokenKubeconfig, found := t.secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameGenericTokenKubeconfig)
	}

	secretCARuntime, found := t.secretsManager.Get(operatorv1alpha1.SecretNameCARuntime)
	if !found {
		return fmt.Errorf("secret %q not found", operatorv1alpha1.SecretNameCARuntime)
	}

	serverCertSecret, err := t.reconcileSecretServerCert(ctx)
	if err != nil {
		return err
	}

	configMap := t.configMap()

	runtimeResources, err := runtimeRegistry.AddAllAndSerialize(
		t.service(),
		configMap,
		t.deployment(secretGenericTokenKubeconfig.Name, virtualGardenAccessSecret.Secret.Name, serverCertSecret.Name, configMap.Name),
		t.podDisruptionBudget(),
		t.verticalPodAutoscaler(),
		t.serviceMonitor(),
	)
	if err != nil {
		return err
	}

	if err := managedresources.CreateForSeedWithLabels(ctx, t.client, t.namespace, managedResourceNameRuntime, false, managedResourceLabels, runtimeResources); err != nil {
		return err
	}

	var (
		virtualRegistry = managedresources.NewRegistry(operatorclient.VirtualScheme, operatorclient.VirtualCodec, operatorclient.VirtualSerializer)
	)

	crd, err := t.crd()
	if err != nil {
		return err
	}

	virtualResources, err := virtualRegistry.AddAllAndSerialize(
		crd,
		t.mutatingWebhookConfiguration(secretCARuntime.Data[secretsutils.DataKeyCertificateBundle]),
		t.validatingWebhookConfiguration(secretCARuntime.Data[secretsutils.DataKeyCertificateBundle]),
		t.clusterRole(),
		t.clusterRoleBinding(virtualGardenAccessSecret.ServiceAccountName),
		t.role(),
		t.roleBinding(virtualGardenAccessSecret.ServiceAccountName),
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForShootWithLabels(ctx, t.client, t.namespace, managedResourceNameVirtual, managedresources.LabelValueGardener, false, managedResourceLabels, virtualResources)
}

func (t *terminal) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return flow.Parallel(
		func(ctx context.Context) error {
			return managedresources.WaitUntilHealthy(ctx, t.client, t.namespace, managedResourceNameRuntime)
		},
		func(ctx context.Context) error {
			return managedresources.WaitUntilHealthy(ctx, t.client, t.namespace, managedResourceNameVirtual)
		},
	)(timeoutCtx)
}

func (t *terminal) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForShoot(ctx, t.client, t.namespace, managedResourceNameVirtual); err != nil {
		return err
	}

	if err := managedresources.DeleteForSeed(ctx, t.client, t.namespace, managedResourceNameRuntime); err != nil {
		return err
	}

	return kubernetesutils.DeleteObjects(ctx, t.client, t.newVirtualGardenAccessSecret().Secret)
}

func (t *terminal) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return flow.Parallel(
		func(ctx context.Context) error {
			return managedresources.WaitUntilDeleted(ctx, t.client, t.namespace, managedResourceNameRuntime)
		},
		func(ctx context.Context) error {
			return managedresources.WaitUntilDeleted(ctx, t.client, t.namespace, managedResourceNameVirtual)
		},
	)(timeoutCtx)
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp: name,
	}
}
