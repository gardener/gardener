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
	shootsystem "github.com/gardener/gardener/pkg/component/shoot/system"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

func (g *garden) runMigrations(ctx context.Context, log logr.Logger) error {
	log.Info("Migrating deprecated failure-domain.beta.kubernetes.io labels to topology.kubernetes.io")
	if err := migrateDeprecatedTopologyLabels(ctx, log, g.mgr.GetClient()); err != nil {
		return err
	}

	log.Info("Migrating ClusterRoleBindings for shoot/adminkubeconfig and shoot/viewerkubeconfig")
	if err := migrateAdminViewerKubeconfigClusterRoleBindings(ctx, log, g.mgr.GetClient()); err != nil {
		return fmt.Errorf("failed migrating ClusterRoleBindings for shoot/adminkubeconfig and shoot/viewerkubeconfig: %w", err)
	}

	return nil
}

// TODO: Remove this function when Kubernetes 1.27 support gets dropped.
func migrateDeprecatedTopologyLabels(ctx context.Context, log logr.Logger, seedClient client.Client) error {
	persistentVolumeList := &corev1.PersistentVolumeList{}
	if err := seedClient.List(ctx, persistentVolumeList); err != nil {
		return fmt.Errorf("failed listing persistent volumes for migrating deprecated topology labels: %w", err)
	}

	var taskFns []flow.TaskFn

	for _, pv := range persistentVolumeList.Items {
		persistentVolume := pv

		taskFns = append(taskFns, func(ctx context.Context) error {
			patch := client.MergeFrom(persistentVolume.DeepCopy())

			if persistentVolume.Spec.NodeAffinity == nil {
				// when PV is very old and has no node affinity, we just replace the topology labels
				if v, ok := persistentVolume.Labels[corev1.LabelFailureDomainBetaRegion]; ok {
					persistentVolume.Labels[corev1.LabelTopologyRegion] = v
				}
				if v, ok := persistentVolume.Labels[corev1.LabelFailureDomainBetaZone]; ok {
					persistentVolume.Labels[corev1.LabelTopologyZone] = v
				}
			} else if persistentVolume.Spec.NodeAffinity.Required != nil {
				// when PV has node affinity then we do not need the labels but just need to replace the topology keys
				// in the node selector term match expressions
				for i, term := range persistentVolume.Spec.NodeAffinity.Required.NodeSelectorTerms {
					for j, expression := range term.MatchExpressions {
						if expression.Key == corev1.LabelFailureDomainBetaRegion {
							persistentVolume.Spec.NodeAffinity.Required.NodeSelectorTerms[i].MatchExpressions[j].Key = corev1.LabelTopologyRegion
						}

						if expression.Key == corev1.LabelFailureDomainBetaZone {
							persistentVolume.Spec.NodeAffinity.Required.NodeSelectorTerms[i].MatchExpressions[j].Key = corev1.LabelTopologyZone
						}
					}
				}
			}

			// either new topology labels were added above, or node affinity keys were adjusted
			// in both cases, the old, deprecated topology labels are no longer needed and can be removed
			delete(persistentVolume.Labels, corev1.LabelFailureDomainBetaRegion)
			delete(persistentVolume.Labels, corev1.LabelFailureDomainBetaZone)

			// prevent sending empty patches
			if data, err := patch.Data(&persistentVolume); err != nil {
				return fmt.Errorf("failed getting patch data for PV %s: %w", persistentVolume.Name, err)
			} else if string(data) == `{}` {
				return nil
			}

			log.Info("Migrating deprecated topology labels", "persistentVolumeName", persistentVolume.Name)
			return seedClient.Patch(ctx, &persistentVolume, patch)
		})
	}

	return flow.Parallel(taskFns...)(ctx)
}

// TODO(@vpnachev): Remove this after v1.127.0 has been released
func migrateAdminViewerKubeconfigClusterRoleBindings(ctx context.Context, log logr.Logger, seedClient client.Client) error {
	namespaceList := &corev1.NamespaceList{}
	if err := seedClient.List(ctx, namespaceList, client.MatchingLabels(map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot})); err != nil {
		return fmt.Errorf("failed listing namespaces: %w", err)
	}

	var (
		tasks     []flow.TaskFn
		crbs      = []string{v1beta1constants.ShootProjectAdminsGroupName, v1beta1constants.ShootProjectViewersGroupName, v1beta1constants.ShootSystemAdminsGroupName, v1beta1constants.ShootSystemViewersGroupName}
		crbsCount = len(crbs)
	)

	for _, namespace := range namespaceList.Items {
		if namespace.DeletionTimestamp != nil || namespace.Status.Phase == corev1.NamespaceTerminating {
			continue
		}

		tasks = append(tasks, func(ctx context.Context) error {
			var (
				key             = client.ObjectKey{Namespace: namespace.Name, Name: shootsystem.ManagedResourceName}
				managedResource = &resourcesv1alpha1.ManagedResource{}
			)

			if err := seedClient.Get(ctx, key, managedResource); err != nil {
				if apierrors.IsNotFound(err) {
					log.Info("Managed resource not found, skipping migration", "managedResource", key)
					return nil
				}
				return fmt.Errorf("failed to get ManagedResource %q: %w", key, err)
			}

			if managedResource.DeletionTimestamp != nil {
				return nil
			}

			objects, err := managedresources.GetObjects(ctx, seedClient, managedResource.Namespace, managedResource.Name)
			if err != nil {
				return fmt.Errorf("failed to get objects for ManagedResource %q: %w", key, err)
			}

			oldObjectsCount := len(objects)
			objects = slices.DeleteFunc(objects, func(obj client.Object) bool {
				return slices.Contains(crbs, obj.GetName())
			})

			if oldObjectsCount-len(objects) == crbsCount {
				log.Info("ClusterRoleBindings for shoot/adminkubeconfig and shoot/viewerkubeconfig have already been migrated, skipping", "managedResource", key)
				return nil
			}

			objects = append(objects, shootsystem.ClusterRoleBindings()...)

			log.Info("Migrating ClusterRoleBindings for shoot/adminkubeconfig and shoot/viewerkubeconfig access in managed resource", "managedResource", key)
			registry := managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
			resources, err := registry.AddAllAndSerialize(objects...)
			if err != nil {
				return fmt.Errorf("failed serializing objects for ManagedResource %q: %w", key, err)
			}

			return managedresources.CreateForShoot(ctx, seedClient, managedResource.Namespace, managedResource.Name, managedresources.LabelValueGardener, false, resources)
		})
	}

	return flow.Parallel(tasks...)(ctx)
}
