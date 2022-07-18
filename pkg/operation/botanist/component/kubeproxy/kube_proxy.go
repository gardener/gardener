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

package kubeproxy

import (
	"context"
	"fmt"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/managedresources"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	labelKeyManagedResourceName   = "component"
	labelValueManagedResourceName = "kube-proxy"
	labelValueRole                = "pool"
	labelKeyPoolName              = "pool-name"
	labelKeyKubernetesVersion     = "kubernetes-version"
)

var (
	labelSelectorManagedResourcesAll = client.MatchingLabels{
		labelKeyManagedResourceName: labelValueManagedResourceName,
	}
	labelSelectorManagedResourcesPoolSpecific = client.MatchingLabels{
		labelKeyManagedResourceName: labelValueManagedResourceName,
		v1beta1constants.LabelRole:  labelValueRole,
	}
)

// New creates a new instance of DeployWaiter for kube-proxy.
func New(
	client client.Client,
	namespace string,
	values Values,
) Interface {
	return &kubeProxy{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

// Interface is an interface for managing kube-proxy DaemonSets.
type Interface interface {
	component.DeployWaiter
	component.MonitoringComponent
	// DeleteStaleResources deletes no longer required ManagedResource from the shoot namespace in the seed.
	DeleteStaleResources(context.Context) error
	// WaitCleanupStaleResources waits until all no longer required ManagedResource are cleaned up.
	WaitCleanupStaleResources(context.Context) error
	// SetKubeconfig sets the Kubeconfig field in the Values.
	SetKubeconfig([]byte)
	// SetWorkerPools sets the WorkerPools field in the Values.
	SetWorkerPools([]WorkerPool)
}

type kubeProxy struct {
	client    client.Client
	namespace string
	values    Values

	serviceAccount              *corev1.ServiceAccount
	secret                      *corev1.Secret
	configMap                   *corev1.ConfigMap
	configMapCleanupScript      *corev1.ConfigMap
	configMapConntrackFixScript *corev1.ConfigMap
}

// Values is a set of configuration values for the kube-proxy component.
type Values struct {
	// IPVSEnabled states whether IPVS is enabled.
	IPVSEnabled bool
	// FeatureGates is the set of feature gates.
	FeatureGates map[string]bool
	// ImageAlpine is the alpine container image.
	ImageAlpine string
	// Kubeconfig is the kubeconfig which should be used to communicate with the kube-apiserver.
	Kubeconfig []byte
	// PodNetworkCIDR is the CIDR of the pod network. Only relevant when IPVSEnabled is false.
	PodNetworkCIDR *string
	// VPAEnabled states whether VerticalPodAutoscaler is enabled.
	VPAEnabled bool
	// WorkerPools is a list of worker pools for which the kube-proxy DaemonSets should be deployed.
	WorkerPools []WorkerPool
	// PSPDisabled marks whether the PodSecurityPolicy admission plugin is disabled.
	PSPDisabled bool
}

// WorkerPool contains configuration for the kube-proxy deployment for this specific worker pool.
type WorkerPool struct {
	// Name is the name of the worker pool.
	Name string
	// KubernetesVersion is the Kubernetes version of the worker pool.
	KubernetesVersion string
	// Image is the container image used for kube-proxy for this worker pool.
	Image string
}

func (k *kubeProxy) Deploy(ctx context.Context) error {
	data, err := k.computeCentralResourcesData()
	if err != nil {
		return err
	}

	if err := k.reconcileManagedResource(ctx, data, nil); err != nil {
		return err
	}

	return k.forEachWorkerPool(ctx, false, func(ctx context.Context, pool WorkerPool) error {
		data, err := k.computePoolResourcesData(pool)
		if err != nil {
			return err
		}

		return k.reconcileManagedResource(ctx, data, &pool)
	})
}

func (k *kubeProxy) reconcileManagedResource(ctx context.Context, data map[string][]byte, pool *WorkerPool) error {
	var (
		mrName             = managedResourceName(pool)
		secretName, secret = managedresources.NewSecret(k.client, k.namespace, mrName, data, true)
		managedResource    = managedresources.NewForShoot(k.client, k.namespace, mrName, false).WithSecretRef(secretName)
	)

	secret = secret.WithLabels(getManagedResourceLabels(pool))
	managedResource = managedResource.WithLabels(getManagedResourceLabels(pool))

	if err := secret.Reconcile(ctx); err != nil {
		return err
	}

	return managedResource.Reconcile(ctx)
}

func (k *kubeProxy) Destroy(ctx context.Context) error {
	return k.forEachExistingManagedResource(ctx, false, labelSelectorManagedResourcesAll, func(ctx context.Context, managedResource resourcesv1alpha1.ManagedResource) error {
		return managedresources.DeleteForShoot(ctx, k.client, k.namespace, managedResource.Name)
	})
}

func (k *kubeProxy) DeleteStaleResources(ctx context.Context) error {
	return k.forEachExistingManagedResource(ctx, false, labelSelectorManagedResourcesPoolSpecific, func(ctx context.Context, managedResource resourcesv1alpha1.ManagedResource) error {
		if k.isExistingManagedResourceStillDesired(managedResource.Labels) {
			return nil
		}
		return managedresources.DeleteForShoot(ctx, k.client, k.namespace, managedResource.Name)
	})
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (k *kubeProxy) Wait(ctx context.Context) error {
	if err := managedresources.WaitUntilHealthy(ctx, k.client, k.namespace, managedResourceName(nil)); err != nil {
		return err
	}

	return k.forEachWorkerPool(ctx, true, func(ctx context.Context, pool WorkerPool) error {
		return managedresources.WaitUntilHealthy(ctx, k.client, k.namespace, managedResourceName(&pool))
	})
}

func (k *kubeProxy) WaitCleanup(ctx context.Context) error {
	return k.forEachExistingManagedResource(ctx, true, labelSelectorManagedResourcesAll, func(ctx context.Context, managedResource resourcesv1alpha1.ManagedResource) error {
		return managedresources.WaitUntilDeleted(ctx, k.client, k.namespace, managedResource.Name)
	})
}

func (k *kubeProxy) WaitCleanupStaleResources(ctx context.Context) error {
	return k.forEachExistingManagedResource(ctx, true, labelSelectorManagedResourcesPoolSpecific, func(ctx context.Context, managedResource resourcesv1alpha1.ManagedResource) error {
		if k.isExistingManagedResourceStillDesired(managedResource.Labels) {
			return nil
		}
		return managedresources.WaitUntilDeleted(ctx, k.client, k.namespace, managedResource.Name)
	})
}

func (k *kubeProxy) forEachWorkerPool(
	ctx context.Context,
	withTimeout bool,
	f func(context.Context, WorkerPool) error,
) error {
	fns := make([]flow.TaskFn, 0, len(k.values.WorkerPools))

	for _, pool := range k.values.WorkerPools {
		p := pool
		fns = append(fns, func(ctx context.Context) error {
			return f(ctx, p)
		})
	}

	return runParallelFunctions(ctx, withTimeout, fns)
}

func (k *kubeProxy) forEachExistingManagedResource(
	ctx context.Context,
	withTimeout bool,
	labelSelector client.MatchingLabels,
	f func(context.Context, resourcesv1alpha1.ManagedResource) error,
) error {
	managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
	if err := k.client.List(ctx, managedResourceList, client.InNamespace(k.namespace), labelSelector); err != nil {
		return err
	}

	fns := make([]flow.TaskFn, 0, len(managedResourceList.Items))

	for _, managedResource := range managedResourceList.Items {
		m := managedResource
		fns = append(fns, func(ctx context.Context) error {
			return f(ctx, m)
		})
	}

	return runParallelFunctions(ctx, withTimeout, fns)
}

func runParallelFunctions(ctx context.Context, withTimeout bool, fns []flow.TaskFn) error {
	parallelCtx := ctx

	if withTimeout {
		timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
		defer cancel()
		parallelCtx = timeoutCtx
	}

	return flow.Parallel(fns...)(parallelCtx)
}

func (k *kubeProxy) isExistingManagedResourceStillDesired(labels map[string]string) bool {
	for _, pool := range k.values.WorkerPools {
		if pool.Name == labels[labelKeyPoolName] && pool.KubernetesVersion == labels[labelKeyKubernetesVersion] {
			return true
		}
	}

	return false
}

func getManagedResourceLabels(pool *WorkerPool) map[string]string {
	labels := map[string]string{
		labelKeyManagedResourceName:     labelValueManagedResourceName,
		managedresources.LabelKeyOrigin: managedresources.LabelValueGardener,
	}

	if pool != nil {
		labels[v1beta1constants.LabelRole] = labelValueRole
		labels[labelKeyPoolName] = pool.Name
		labels[labelKeyKubernetesVersion] = pool.KubernetesVersion
	}

	return labels
}

func managedResourceName(pool *WorkerPool) string {
	if pool == nil {
		return "shoot-core-kube-proxy"
	}
	return fmt.Sprintf("shoot-core-%s", name(*pool))
}

func name(pool WorkerPool) string {
	return fmt.Sprintf("kube-proxy-%s-v%s", pool.Name, pool.KubernetesVersion)
}

func (k *kubeProxy) SetKubeconfig(kubeconfig []byte)   { k.values.Kubeconfig = kubeconfig }
func (k *kubeProxy) SetWorkerPools(pools []WorkerPool) { k.values.WorkerPools = pools }
