// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"slices"

	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	shootsystem "github.com/gardener/gardener/pkg/component/shoot/system"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

func (g *garden) runMigrations(ctx context.Context, log logr.Logger) error {
	log.Info("Removing Prometheus cleaned up obsolete folder annotations")
	if err := removePrometheusFolderCleanedupAnnotation(ctx, log, g.mgr.GetClient()); err != nil {
		return fmt.Errorf("failed removing Prometheus cleaned up obsolete folder annotations: %w", err)
	}

	log.Info("Migrating ClusterRoleBindings for shoot/adminkubeconfig and shoot/viewerkubeconfig")
	if err := migrateAdminViewerKubeconfigClusterRoleBindings(ctx, log, g.mgr.GetClient()); err != nil {
		return fmt.Errorf("failed migrating ClusterRoleBindings for shoot/adminkubeconfig and shoot/viewerkubeconfig: %w", err)
	}

	return nil
}

// TODO(vicwicker): Remove this after v1.128 has been released.
func removePrometheusFolderCleanedupAnnotation(ctx context.Context, log logr.Logger, seedClient client.Client) error {
	var tasks []flow.TaskFn

	getPrometheusWithPatch := func(ctx context.Context, namespace string) (*monitoringv1.Prometheus, client.Patch, error) {
		prometheus := &monitoringv1.Prometheus{}
		if err := seedClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "shoot"}, prometheus); err != nil {
			return nil, nil, err
		}

		return prometheus, client.MergeFrom(prometheus.DeepCopy()), nil
	}

	shouldSkipCluster := func(ctx context.Context, log logr.Logger, cluster *extensionsv1alpha1.Cluster) (bool, error) {
		shoot, err := extensions.ShootFromCluster(cluster)
		if err != nil {
			return false, fmt.Errorf("failed to extract Shoot from Cluster %s: %w", cluster.Name, err)
		}

		if shoot.DeletionTimestamp != nil {
			log.Info("Cluster is being deleted, it should be skipped", "cluster", cluster.Name)
			return true, nil
		}

		namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: cluster.Name}}
		if err := seedClient.Get(ctx, client.ObjectKeyFromObject(namespace), namespace); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("Namespace for cluster not found, cluster should be skipped", "cluster", cluster.Name)
				return true, nil
			}
			return false, fmt.Errorf("failed to get Namespace for cluster %s: %w", cluster.Name, err)
		}

		if namespace.DeletionTimestamp != nil {
			log.Info("Namespace for cluster is being deleted, cluster should be skipped", "cluster", cluster.Name)
			return true, nil
		}

		return false, nil
	}

	log.Info("Remove folder cleaned up annotations from Prometheus")

	// check if the Cluster resource is available in the seed cluster
	gvk := schema.GroupVersionKind{
		Group:   "extensions.gardener.cloud",
		Version: "v1alpha1",
		Kind:    "Cluster",
	}

	if _, err := seedClient.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version); err != nil {
		if meta.IsNoMatchError(err) {
			log.Info("The Cluster resource is not available in the extensions.gardener.cloud/v1alpha1 API group")
			return nil
		}
		return fmt.Errorf("failed to check if the Cluster resource is available in the extensions.gardener.cloud/v1alpha1 API group: %w", err)
	}

	clusterList := &extensionsv1alpha1.ClusterList{}
	if err := seedClient.List(ctx, clusterList); err != nil {
		return fmt.Errorf("failed to list clusters for annotation removal from Prometheus: %w", err)
	}

	for _, cluster := range clusterList.Items {
		tasks = append(tasks, func(ctx context.Context) error {
			skip, err := shouldSkipCluster(ctx, log, &cluster)
			if err != nil {
				return err
			}

			if skip {
				log.Info("Skip annotation removal for cluster", "cluster", cluster.Name)
				return nil
			}

			prometheus, prometheusPatch, err := getPrometheusWithPatch(ctx, cluster.Name)
			if err != nil {
				if apierrors.IsNotFound(err) {
					log.Info("Prometheus resource not found, skipping annotation removal", "cluster", cluster.Name)
					return nil
				}
				return fmt.Errorf("failed to get Prometheus resource for cluster %s: %w", cluster.Name, err)
			}

			if _, ok := prometheus.Annotations[resourcesv1alpha1.PrometheusObsoleteFolderCleanedUp]; !ok {
				// annotation already removed, nothing to do
				return nil
			}

			delete(prometheus.Annotations, resourcesv1alpha1.PrometheusObsoleteFolderCleanedUp)
			if err := seedClient.Patch(ctx, prometheus, prometheusPatch); err != nil {
				return fmt.Errorf("failed to remove annotation from Prometheus resource for cluster %s: %w", cluster.Name, err)
			}

			return nil
		})
	}

	return flow.Parallel(tasks...)(ctx)
}

// TODO(vpnachev): Remove this after v1.128.0 has been released.
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
				if apierrors.IsNotFound(err) {
					log.Info("Managed resource secret not found, skipping migration", "managedResource", key)
					return nil
				}
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
