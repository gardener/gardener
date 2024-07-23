// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubestatemetrics

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	appsv1 "k8s.io/api/apps/v1"
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
		registry                         = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
		deployment                       *appsv1.Deployment
		customResourceStateConfigMap     = k.customResourceStateConfigMap()
	)

	// TODO(chrkl): Remove after release v1.103
	if k.values.ClusterType == component.ClusterTypeSeed && k.values.NameSuffix != "" {
		if err := component.DestroyResourceConfigs(ctx, k.client, k.namespace, k.values.ClusterType, managedResourceName, nil); client.IgnoreNotFound(err) != nil {
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

	if k.values.ClusterType == component.ClusterTypeSeed {
		clusterRole := k.clusterRole()
		serviceAccount := k.serviceAccount()
		deployment = k.deployment(serviceAccount, "", nil, customResourceStateConfigMap.Name)
		resources := []client.Object{
			clusterRole,
			serviceAccount,
			k.clusterRoleBinding(clusterRole, serviceAccount),
			deployment,
			k.podDisruptionBudget(deployment),
		}

		if k.values.NameSuffix == SuffixSeed {
			resources = append(
				resources,
				k.scrapeConfigSeed(),
				k.scrapeConfigCache(),
			)
		} else if k.values.NameSuffix == SuffixRuntime {
			resources = append(
				resources,
				k.scrapeConfigGarden(),
			)
		}

		if err := registry.Add(resources...); err != nil {
			return err
		}
	}

	if k.values.ClusterType == component.ClusterTypeShoot {
		deployment = k.deployment(nil, genericTokenKubeconfigSecretName, shootAccessSecret, customResourceStateConfigMap.Name)
		if err := registry.Add(
			deployment,
			k.prometheusRuleShoot()); err != nil {
			return err
		}

		if !k.values.IsWorkerless {
			if err := registry.Add(k.scrapeConfigShoot()); err != nil {
				return err
			}
		}
	}

	serializedResources, err := registry.AddAllAndSerialize(
		k.service(),
		k.verticalPodAutoscaler(deployment),
		customResourceStateConfigMap,
	)

	if err != nil {
		return err
	}

	if err := managedresources.CreateForSeedWithLabels(ctx,
		k.client,
		k.namespace,
		k.managedResourceName(),
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

		// TODO(vicwicker): Remove after KSM upgrade to 2.13.
		if err := managedresources.WaitUntilHealthyAndNotProgressing(ctx, k.client, k.namespace, k.managedResourceName()); err != nil {
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

	return nil
}

func (k *kubeStateMetrics) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForSeed(ctx, k.client, k.namespace, k.managedResourceName()); err != nil {
		return err
	}

	if k.values.ClusterType == component.ClusterTypeShoot {
		if err := managedresources.DeleteForShoot(ctx, k.client, k.namespace, k.managedResourceName()+"-target"); err != nil {
			return err
		}

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
