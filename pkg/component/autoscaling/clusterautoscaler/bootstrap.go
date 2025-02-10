// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package clusterautoscaler

import (
	"context"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	clusterRoleControlName     = "system:cluster-autoscaler-seed"
	managedResourceControlName = "cluster-autoscaler"
)

// NewBootstrapper creates a new instance of DeployWaiter for the cluster-autoscaler bootstrapper.
func NewBootstrapper(client client.Client, namespace string) component.DeployWaiter {
	return &bootstrapper{
		client:    client,
		namespace: namespace,
	}
}

type bootstrapper struct {
	client    client.Client
	namespace string
}

func (b *bootstrapper) Deploy(ctx context.Context) error {
	var (
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterRoleControlName,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{machinev1alpha1.GroupName},
					Resources: []string{"machineclasses", "machinedeployments", "machines", "machinesets"},
					Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{"apps"},
					Resources: []string{"deployments"},
					Verbs:     []string{"get", "list", "watch"},
				},
			},
		}
	)

	resources, err := registry.AddAllAndSerialize(clusterRole)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, b.client, b.namespace, managedResourceControlName, false, resources)
}

func (b *bootstrapper) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, b.client, b.namespace, managedResourceControlName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (b *bootstrapper) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, b.client, b.namespace, managedResourceControlName)
}

func (b *bootstrapper) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, b.client, b.namespace, managedResourceControlName)
}
