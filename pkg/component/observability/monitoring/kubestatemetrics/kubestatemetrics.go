// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubestatemetrics

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	managedResourceName      = "kube-state-metrics"
	managedResourceNameShoot = "shoot-core-" + managedResourceName

	containerName = "kube-state-metrics"

	labelKeyComponent   = "component"
	labelKeyType        = "type"
	labelValueComponent = "kube-state-metrics"

	port            = 8080
	portNameMetrics = "metrics"

	// SuffixSeed is the suffix for seed kube-state-metrics resources.
	SuffixSeed = "-seed"
	// SuffixRuntime is the suffix for garden-runtime kube-state-metrics resources.
	SuffixRuntime = "-runtime"
)

// New creates a new instance of DeployWaiter for the kube-state-metrics.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) component.DeployWaiter {
	return &kubeStateMetrics{
		client:         client,
		secretsManager: secretsManager,
		namespace:      namespace,
		values:         values,
	}
}

type kubeStateMetrics struct {
	client         client.Client
	secretsManager secretsmanager.Interface
	namespace      string
	values         Values
}

// Values is a set of configuration values for the kube-state-metrics.
type Values struct {
	// ClusterType specifies the type of the cluster to which kube-state-metrics is being deployed.
	// For seeds, all resources are being deployed as part of a ManagedResource.
	// For shoots, the kube-state-metrics runs in the shoot namespace in the seed as part of the control plane. Hence,
	// only the runtime resources (like Deployment, Service, etc.) are being deployed directly (with the client). All
	// other application-related resources (like RBAC roles, CRD, etc.) are deployed as part of a ManagedResource.
	ClusterType component.ClusterType
	// KubernetesVersion is the Kubernetes version of the cluster.
	KubernetesVersion *semver.Version
	// Image is the container image.
	Image string
	// PriorityClassName is the name of the priority class.
	PriorityClassName string
	// Replicas is the number of replicas.
	Replicas int32
	// IsWorkerless specifies whether the cluster has worker nodes.
	IsWorkerless bool
	// NameSuffix is attached to the deployment name and related resources.
	NameSuffix string
}

func (k *kubeStateMetrics) Deploy(ctx context.Context) error {
	var (
		genericTokenKubeconfigSecretName string
		shootAccessSecret                *gardenerutils.AccessSecret
		registry2                        = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
	)

	// TODO(chrkl): Remove after release v1.103
	if k.values.ClusterType == component.ClusterTypeSeed && k.values.NameSuffix != "" {
		if err := component.DestroyResourceConfigs(ctx, k.client, k.namespace, k.values.ClusterType, managedResourceName, k.getResourceConfigs("", nil)); client.IgnoreNotFound(err) != nil {
			return err
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
		defer cancel()
		if err := managedresources.WaitUntilDeleted(timeoutCtx, k.client, k.namespace, managedResourceName); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

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

	var registry *managedresources.Registry
	if k.values.ClusterType == component.ClusterTypeSeed {
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
	} else {
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
	}

	if k.values.ClusterType == component.ClusterTypeSeed {
		clusterRole := k.clusterRole()
		serviceAccount := k.serviceAccount()
		if err := registry2.Add(
			clusterRole,
			serviceAccount,
			k.clusterRoleBinding(clusterRole, serviceAccount)); err != nil {
			return err
		}
	}

	serializedResources, err := registry2.AddAllAndSerialize(k.service())
	if err != nil {
		return err
	}

	if err := managedresources.CreateForSeedWithLabels(ctx,
		k.client,
		k.namespace,
		k.managedResourceName()+"-2",
		false,
		map[string]string{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy},
		serializedResources,
	); err != nil {
		return err
	}

	if k.values.ClusterType == component.ClusterTypeShoot {
		registryTarget := managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
		clusterRole := k.clusterRole()
		resourcesTarget, err := registryTarget.AddAllAndSerialize(
			clusterRole,
			k.clusterRoleBinding(clusterRole, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: shootAccessSecret.ServiceAccountName, Namespace: metav1.NamespaceSystem}}),
		)
		if err != nil {
			return err
		}

		return managedresources.CreateForShootWithLabels(ctx,
			k.client,
			k.namespace,
			k.managedResourceName()+"-target",
			managedresources.LabelValueGardener,
			false,
			map[string]string{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy},
			resourcesTarget,
		)
	}

	return component.DeployResourceConfigs(ctx, k.client, k.namespace, k.values.ClusterType, k.managedResourceName(), map[string]string{v1beta1constants.LabelCareConditionType: v1beta1constants.ObservabilityComponentsHealthy}, registry, k.getResourceConfigs(genericTokenKubeconfigSecretName, shootAccessSecret))
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
		return managedResourceName + k.values.NameSuffix
	}
	return managedResourceNameShoot + k.values.NameSuffix
}
