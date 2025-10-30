// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"slices"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardeneraccess "github.com/gardener/gardener/pkg/component/gardener/access"
	shootsystem "github.com/gardener/gardener/pkg/component/shoot/system"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

func (g *garden) runMigrations(ctx context.Context, log logr.Logger) error {
	log.Info("Migrating ClusterRoleBindings for shoot/adminkubeconfig and shoot/viewerkubeconfig")
	if err := migrateAdminViewerKubeconfigClusterRoleBindings(ctx, log, g.mgr.GetClient()); err != nil {
		return fmt.Errorf("failed migrating ClusterRoleBindings for shoot/adminkubeconfig and shoot/viewerkubeconfig: %w", err)
	}

	return nil
}

// migrateAdminViewerKubeconfigClusterRoleBindings moves the ClusterRoleBindings granting access to the
// shoot/adminkubeconfig and shoot/viewerkubeconfig subresources from the shoot-core-system managed resource to the
// shoot-core-gardeneraccess managed resource.
// TODO(vpnachev): Remove this after v1.133.0 has been released.
func migrateAdminViewerKubeconfigClusterRoleBindings(ctx context.Context, log logr.Logger, seedClient client.Client) error {
	namespaceList := &corev1.NamespaceList{}
	if err := seedClient.List(ctx, namespaceList, client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot}); err != nil {
		return fmt.Errorf("failed listing namespaces: %w", err)
	}

	var (
		tasks []flow.TaskFn
		crbs  = []string{v1beta1constants.ShootProjectAdminsGroupName, v1beta1constants.ShootProjectViewersGroupName, v1beta1constants.ShootSystemAdminsGroupName, v1beta1constants.ShootSystemViewersGroupName}
	)

	for _, namespace := range namespaceList.Items {
		if namespace.DeletionTimestamp != nil || namespace.Status.Phase == corev1.NamespaceTerminating {
			continue
		}

		tasks = append(tasks, func(ctx context.Context) error {
			var (
				shootSystemKey             = client.ObjectKey{Namespace: namespace.Name, Name: shootsystem.ManagedResourceName}
				shootSystemManagedResource = &resourcesv1alpha1.ManagedResource{}

				gardenerAccessKey             = client.ObjectKey{Namespace: namespace.Name, Name: gardeneraccess.ManagedResourceName}
				gardenerAccessManagedResource = &resourcesv1alpha1.ManagedResource{}
			)

			// Get the shoot-core-system managed resource and check if it is already migrated.
			if err := seedClient.Get(ctx, shootSystemKey, shootSystemManagedResource); err != nil {
				if apierrors.IsNotFound(err) {
					log.Info("Managed resource not found, skipping migration", "managedResource", shootSystemKey)
					return nil
				}
				return fmt.Errorf("failed to get ManagedResource %q: %w", shootSystemKey, err)
			}

			if shootSystemManagedResource.DeletionTimestamp != nil {
				log.Info("Managed resource is in deletion, skipping migration", "managedResource", shootSystemKey)
				return nil
			}

			shootSystemObjects, err := managedresources.GetObjects(ctx, seedClient, shootSystemManagedResource.Namespace, shootSystemManagedResource.Name)
			if err != nil {
				if apierrors.IsNotFound(err) {
					log.Info("Managed resource secret not found, skipping migration", "managedResource", shootSystemKey)
					return nil
				}
				return fmt.Errorf("failed to get objects for ManagedResource %q: %w", shootSystemKey, err)
			}

			oldShootSystemObjectsCount := len(shootSystemObjects)
			shootSystemObjects = slices.DeleteFunc(shootSystemObjects, func(obj client.Object) bool {
				return slices.Contains(crbs, obj.GetName())
			})

			if oldShootSystemObjectsCount == len(shootSystemObjects) {
				log.Info("ClusterRoleBindings for shoot/adminkubeconfig and shoot/viewerkubeconfig have already been migrated, skipping migration", "managedResource", shootSystemKey)
				return nil
			}

			// Move the ClusterRoleBindings to the shoot-core-gardeneraccess managed resource firstly.
			if err := seedClient.Get(ctx, gardenerAccessKey, gardenerAccessManagedResource); err != nil {
				if apierrors.IsNotFound(err) {
					log.Info("Managed resource not found, skipping migration", "managedResource", gardenerAccessKey)
					return nil
				}
				return fmt.Errorf("failed to get ManagedResource %q: %w", gardenerAccessKey, err)
			}

			if gardenerAccessManagedResource.DeletionTimestamp != nil {
				log.Info("Managed resource is in deletion, skipping migration", "managedResource", gardenerAccessKey)
				return nil
			}

			gardenerAccessObjects, err := managedresources.GetObjects(ctx, seedClient, gardenerAccessManagedResource.Namespace, gardenerAccessManagedResource.Name)
			if err != nil {
				if apierrors.IsNotFound(err) {
					log.Info("Managed resource secret not found, skipping migration", "managedResource", gardenerAccessKey)
					return nil
				}
				return fmt.Errorf("failed to get objects for ManagedResource %q: %w", gardenerAccessKey, err)
			}

			gardenerAccessObjects = append(gardenerAccessObjects, gardeneraccess.ShootAccessClusterRoleBindings()...)
			gardenerAccessRegistry := managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
			gardenerAccessResources, err := gardenerAccessRegistry.AddAllAndSerialize(gardenerAccessObjects...)
			if err != nil {
				return fmt.Errorf("failed serializing objects for ManagedResource %q: %w", gardenerAccessKey, err)
			}

			if err := managedresources.CreateForShoot(ctx, seedClient, gardenerAccessManagedResource.Namespace, gardenerAccessManagedResource.Name, managedresources.LabelValueGardener, false, gardenerAccessResources); err != nil {
				return fmt.Errorf("failed updating ManagedResource %q: %w", gardenerAccessKey, err)
			}

			// Remove the ClusterRoleBindings from the shoot-core-system managed resource.
			log.Info("Updating ManagedResource status to remove migrated ClusterRoleBindings", "managedResource", shootSystemKey)
			patch := client.MergeFrom(shootSystemManagedResource.DeepCopy())
			shootSystemManagedResource.Status.Resources = slices.DeleteFunc(shootSystemManagedResource.Status.Resources, func(objRef resourcesv1alpha1.ObjectReference) bool {
				return objRef.APIVersion == "rbac.authorization.k8s.io/v1" && objRef.Kind == "ClusterRoleBinding" && slices.Contains(crbs, objRef.Name)
			})

			if err := seedClient.Status().Patch(ctx, shootSystemManagedResource, patch); err != nil {
				return fmt.Errorf("failed updating status of ManagedResource %q: %w", shootSystemKey, err)
			}

			log.Info("Cleaning ClusterRoleBindings for shoot/adminkubeconfig and shoot/viewerkubeconfig access in managed resource", "managedResource", shootSystemKey)
			shootSystemRegistry := managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
			shootSystemResources, err := shootSystemRegistry.AddAllAndSerialize(shootSystemObjects...)
			if err != nil {
				return fmt.Errorf("failed serializing objects for ManagedResource %q: %w", shootSystemKey, err)
			}
			if err := managedresources.CreateForShoot(ctx, seedClient, shootSystemManagedResource.Namespace, shootSystemManagedResource.Name, managedresources.LabelValueGardener, false, shootSystemResources); err != nil {
				return fmt.Errorf("failed updating ManagedResource %q: %w", shootSystemKey, err)
			}

			return nil
		})
	}

	return flow.Parallel(tasks...)(ctx)
}
