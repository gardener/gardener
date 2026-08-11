// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package adminaccess

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// ManagedResourceName is the name of the ManagedResource containing the resource specifications.
	ManagedResourceName = "shoot-core-adminaccess"
	// ShootAccessSecretNameSuffix is the suffix name of the shoot access secret.
	ShootAccessSecretNameSuffix = "cluster-admin"
	// PathOnControlPlaneNodes is the path on control plane nodes to which the token of this shoot access secret should
	// be synced.
	PathOnControlPlaneNodes = "/etc/kubernetes/admin-token"
)

// New creates a new instance of the deployer for AdminAccess.
func New(client client.Client, namespace string) component.DeployWaiter {
	return &adminAccess{
		client:    client,
		namespace: namespace,
	}
}

type adminAccess struct {
	client    client.Client
	namespace string
}

func (a *adminAccess) Deploy(ctx context.Context) error {
	shootAccessSecret := gardenerutils.NewShootAccessSecret(ShootAccessSecretNameSuffix, a.namespace)
	if err := shootAccessSecret.Reconcile(ctx, a.client); err != nil {
		return err
	}

	data, err := a.computeResourcesData(shootAccessSecret.ServiceAccountName)
	if err != nil {
		return err
	}

	return managedresources.CreateForShootWithLabels(ctx, a.client, a.namespace, ManagedResourceName, managedresources.LabelValueGardener, true, nil, data)
}

func (a *adminAccess) Destroy(ctx context.Context) error {
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: gardenerutils.SecretNamePrefixShootAccess + ShootAccessSecretNameSuffix, Namespace: a.namespace}}
	if err := a.client.Delete(ctx, secret); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed deleting secret %s: %w", client.ObjectKeyFromObject(secret), err)
	}

	return managedresources.DeleteForShoot(ctx, a.client, a.namespace, ManagedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (a *adminAccess) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, a.client, a.namespace, ManagedResourceName)
}

func (a *adminAccess) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, a.client, a.namespace, ManagedResourceName)
}

func (a *adminAccess) computeResourcesData(serviceAccountName string) (map[string][]byte, error) {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		gardenerSystemClusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:system:cluster-admin",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     "cluster-admin",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: metav1.NamespaceSystem,
			}},
		}
	)

	return registry.AddAllAndSerialize(gardenerSystemClusterRoleBinding)
}
