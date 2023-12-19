// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package machinecontrollermanager

import (
	"context"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	managedResourceControlName = "machine-controller-manager"
	clusterRoleName            = "system:machine-controller-manager-runtime"
	// TODO(himanshu-kun): remove after g/g v1.88 has been released
	unsupportedClusterRoleName = "system:machine-controller-manager-seed"
)

// NewBootstrapper creates a new instance of DeployWaiter for the machine-controller-manager bootstrapper.
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
				Name: clusterRoleName,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{machinev1alpha1.GroupName},
					Resources: []string{"*"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{corev1.GroupName},
					Resources: []string{"configmaps", "secrets", "endpoints", "events", "pods"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{coordinationv1.GroupName},
					Resources: []string{"leases"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups:     []string{coordinationv1.GroupName},
					Resources:     []string{"leases"},
					Verbs:         []string{"get", "watch", "update"},
					ResourceNames: []string{"machine-controller", "machine-controller-manager"},
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
