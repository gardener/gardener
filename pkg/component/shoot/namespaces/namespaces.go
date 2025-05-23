// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package namespaces

import (
	"context"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const managedResourceName = "shoot-core-namespaces"

// New creates a new instance of DeployWaiter for the namespaces.
func New(
	client client.Client,
	namespace string,
	workerPools []gardencorev1beta1.Worker,
) component.DeployWaiter {
	return &namespaces{
		client:      client,
		namespace:   namespace,
		workerPools: workerPools,
	}
}

type namespaces struct {
	client      client.Client
	namespace   string
	workerPools []gardencorev1beta1.Worker
}

func (n *namespaces) Deploy(ctx context.Context) error {
	data, err := n.computeResourcesData()
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, n.client, n.namespace, managedResourceName, managedresources.LabelValueGardener, true, data)
}

func (n *namespaces) Destroy(ctx context.Context) error {
	return managedresources.DeleteForShoot(ctx, n.client, n.namespace, managedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (n *namespaces) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, n.client, n.namespace, managedResourceName)
}

func (n *namespaces) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, n.client, n.namespace, managedResourceName)
}

func (n *namespaces) computeResourcesData() (map[string][]byte, error) {
	zones := sets.New[string]()

	for _, pool := range n.workerPools {
		if v1beta1helper.SystemComponentsAllowed(&pool) {
			zones.Insert(pool.Zones...)
		}
	}

	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		kubeSystemNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: metav1.NamespaceSystem,
				Labels: map[string]string{
					v1beta1constants.GardenerPurpose:                 metav1.NamespaceSystem,
					resourcesv1alpha1.HighAvailabilityConfigConsider: "true",
				},
				Annotations: map[string]string{
					resourcesv1alpha1.HighAvailabilityConfigZones: strings.Join(sets.List(zones), ","),
				},
			},
		}
	)

	return registry.AddAllAndSerialize(kubeSystemNamespace)
}
