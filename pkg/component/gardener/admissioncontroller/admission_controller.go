// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package admissioncontroller

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"sigs.k8s.io/controller-runtime/pkg/client"

	admissioncontrollerconfigv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
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
	DeploymentName = "gardener-admission-controller"
	// ServiceName is the name of the service.
	ServiceName = DeploymentName

	roleName = "admission-controller"

	serverPort  = 2719
	probePort   = 2722
	metricsPort = 2723

	// ManagedResourceNameRuntime is the name of the ManagedResource for the runtime resources.
	ManagedResourceNameRuntime = "gardener-admission-controller-runtime"
	// ManagedResourceNameVirtual is the name of the ManagedResource for the virtual resources.
	ManagedResourceNameVirtual = "gardener-admission-controller-virtual"
)

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy or
// deleted.
var TimeoutWaitForManagedResource = 5 * time.Minute

// Values contains configuration values for the gardener-admission-controller resources.
type Values struct {
	// LogLevel is the configured log level for the gardener-admission-controller.
	LogLevel string
	// Image is the container image used for the gardener-admission-controller pods.
	Image string
	// ResourceAdmissionConfiguration is the configuration for gardener-admission-controller's resource-size validator.
	ResourceAdmissionConfiguration *admissioncontrollerconfigv1alpha1.ResourceAdmissionConfiguration
	// RuntimeVersion is the Kubernetes version of the runtime cluster.
	RuntimeVersion *semver.Version
	// SeedRestrictionEnabled specifies whether the seed-restriction webhook is enabled.
	SeedRestrictionEnabled bool
	// TopologyAwareRoutingEnabled determines whether topology aware hints are intended for the gardener-admission-controller.
	TopologyAwareRoutingEnabled bool
}

// New creates a new instance of DeployWaiter for the gardener-admission-controller.
func New(client client.Client, namespace string, secretsManager secretsmanager.Interface, values Values) component.DeployWaiter {
	return &gardenerAdmissionController{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type gardenerAdmissionController struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

func (a *gardenerAdmissionController) Deploy(ctx context.Context) error {
	var (
		runtimeRegistry           = managedresources.NewRegistry(operatorclient.RuntimeScheme, operatorclient.RuntimeCodec, operatorclient.RuntimeSerializer)
		virtualGardenAccessSecret = a.newVirtualGardenAccessSecret()
		managedResourceLabels     = map[string]string{v1beta1constants.LabelCareConditionType: string(operatorv1alpha1.VirtualComponentsHealthy)}
	)

	secretServerCert, err := a.reconcileSecretServerCert(ctx)
	if err != nil {
		return err
	}

	if err := virtualGardenAccessSecret.Reconcile(ctx, a.client); err != nil {
		return err
	}

	admissionConfigConfigMap, err := a.admissionConfigConfigMap()
	if err != nil {
		return err
	}

	secretGenericTokenKubeconfig, found := a.secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameGenericTokenKubeconfig)
	}

	runtimeResources, err := runtimeRegistry.AddAllAndSerialize(
		a.deployment(secretServerCert.Name, secretGenericTokenKubeconfig.Name, virtualGardenAccessSecret.Secret.Name, admissionConfigConfigMap.Name),
		a.podDisruptionBudget(),
		a.service(),
		a.vpa(),
		admissionConfigConfigMap,
		a.serviceMonitor(),
	)
	if err != nil {
		return err
	}

	if err := managedresources.CreateForSeedWithLabels(ctx, a.client, a.namespace, ManagedResourceNameRuntime, false, managedResourceLabels, runtimeResources); err != nil {
		return err
	}

	var virtualRegistry = managedresources.NewRegistry(operatorclient.VirtualScheme, operatorclient.VirtualCodec, operatorclient.VirtualSerializer)

	caSecret, found := a.secretsManager.Get(operatorv1alpha1.SecretNameCAGardener)
	if !found {
		return fmt.Errorf("secret %q not found", operatorv1alpha1.SecretNameCAGardener)
	}

	virtualResources, err := virtualRegistry.AddAllAndSerialize(
		a.clusterRole(),
		a.clusterRoleBinding(virtualGardenAccessSecret.ServiceAccountName),
		a.validatingWebhookConfiguration(caSecret),
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForShootWithLabels(ctx, a.client, a.namespace, ManagedResourceNameVirtual, managedresources.LabelValueGardener, false, managedResourceLabels, virtualResources)
}

func (a *gardenerAdmissionController) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return flow.Parallel(
		func(ctx context.Context) error {
			return managedresources.WaitUntilHealthy(ctx, a.client, a.namespace, ManagedResourceNameRuntime)
		},
		func(ctx context.Context) error {
			return managedresources.WaitUntilHealthy(ctx, a.client, a.namespace, ManagedResourceNameVirtual)
		},
	)(timeoutCtx)
}

func (a *gardenerAdmissionController) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForShoot(ctx, a.client, a.namespace, ManagedResourceNameVirtual); err != nil {
		return err
	}

	if err := managedresources.DeleteForSeed(ctx, a.client, a.namespace, ManagedResourceNameRuntime); err != nil {
		return err
	}

	return kubernetesutils.DeleteObjects(ctx, a.client, a.newVirtualGardenAccessSecret().Secret)
}

func (a *gardenerAdmissionController) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return flow.Parallel(
		func(ctx context.Context) error {
			return managedresources.WaitUntilDeleted(ctx, a.client, a.namespace, ManagedResourceNameRuntime)
		},
		func(ctx context.Context) error {
			return managedresources.WaitUntilDeleted(ctx, a.client, a.namespace, ManagedResourceNameVirtual)
		},
	)(timeoutCtx)
}

// GetLabels returns the labels for the gardener-admission-controller.
func GetLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelGardener,
		v1beta1constants.LabelRole: roleName,
	}
}
