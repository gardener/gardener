// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/nodemanagement/machinecontrollermanager"
	shootsystem "github.com/gardener/gardener/pkg/component/shoot/system"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/retry"
)

func (g *garden) runMigrations(ctx context.Context, log logr.Logger) error {
	log.Info("Migrating deprecated failure-domain.beta.kubernetes.io labels to topology.kubernetes.io")
	if err := migrateDeprecatedTopologyLabels(ctx, log, g.mgr.GetClient()); err != nil {
		return err
	}

	log.Info("Migrating RBAC resources for machine-controller-manager")
	if err := migrateMCMRBAC(ctx, g.mgr.GetClient()); err != nil {
		return err
	}

	log.Info("Cleaning up ingress controller resource lock configmaps")
	if err := cleanupNginxConfigMaps(ctx, g.mgr.GetClient()); err != nil {
		return fmt.Errorf("failed deleting nginx ingress controller resource lock configmaps: %w", err)
	}

	log.Info("Cleaning up prometheus obsolete folders")
	if err := cleanupPrometheusObsoleteFolders(ctx, log, g.mgr.GetClient()); err != nil {
		return fmt.Errorf("failed cleaning up prometheus obsolete folders: %w", err)
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

// TODO(@aaronfern): Remove this after v1.123 is released
func migrateMCMRBAC(ctx context.Context, seedClient client.Client) error {
	namespaceList := &corev1.NamespaceList{}
	if err := seedClient.List(ctx, namespaceList, client.MatchingLabels(map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot})); err != nil {
		return fmt.Errorf("failed listing namespaces with '%s: %s' label: %w", v1beta1constants.GardenRole, v1beta1constants.GardenRoleShoot, err)
	}

	var tasks []flow.TaskFn

	for _, namespace := range namespaceList.Items {
		if namespace.DeletionTimestamp != nil || namespace.Status.Phase == corev1.NamespaceTerminating {
			continue
		}
		tasks = append(tasks, func(ctx context.Context) error {
			clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
			if err := seedClient.Get(ctx, client.ObjectKey{Name: "machine-controller-manager-" + namespace.Name}, clusterRoleBinding); err != nil {
				//If MCM clusterRoleBinding does not exist, nothing to do
				return client.IgnoreNotFound(err)
			}

			return machinecontrollermanager.New(seedClient, namespace.Name, nil, machinecontrollermanager.Values{}).MigrateRBAC(ctx)
		})
	}

	if err := flow.Parallel(tasks...)(ctx); err != nil {
		return err
	}
	if err := managedresources.DeleteForSeed(ctx, seedClient, "garden", "machine-controller-manager"); err != nil {
		if !meta.IsNoMatchError(err) {
			return err
		}
	}
	return nil
}

// TODO(shafeeqes): Remove this function in gardener v1.125
func cleanupNginxConfigMaps(ctx context.Context, client client.Client) error {
	return kubernetesutils.DeleteObjects(
		ctx,
		client,
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ingress-controller-seed-leader",
				Namespace: v1beta1constants.GardenNamespace,
			},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ingress-controller-seed-leader-nginx-gardener",
				Namespace: v1beta1constants.GardenNamespace,
			},
		},
	)
}

// TODO(vicwicker): Remove this after v1.125 has been released.
func cleanupPrometheusObsoleteFolders(ctx context.Context, log logr.Logger, seedClient client.Client) error {
	var tasks []flow.TaskFn

	getManagedResourceWithPatch := func(ctx context.Context, namespace string) (*resourcesv1alpha1.ManagedResource, client.Patch, error) {
		managedResource := &resourcesv1alpha1.ManagedResource{}
		if err := seedClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "prometheus-shoot"}, managedResource); err != nil {
			return nil, nil, err
		}

		return managedResource, client.MergeFrom(managedResource.DeepCopy()), nil
	}

	unignoreManagedResource := func(ctx context.Context, log logr.Logger, cluster string) error {
		managedResource, managedResourcePatch, err := getManagedResourceWithPatch(ctx, cluster)
		if err != nil {
			// tolerate if the managed resource does not exist, e.g., it might have been deleted
			if apierrors.IsNotFound(err) {
				log.Info("ManagedResource for Prometheus not found, nothing to unignore", "cluster", cluster)
				return nil
			}
			return fmt.Errorf("failed to get ManagedResource for Prometheus for cluster %s, it won't be unignored: %w", cluster, err)
		}

		if value, ok := managedResource.Annotations[resourcesv1alpha1.Ignore]; ok && value == "true" {
			delete(managedResource.Annotations, resourcesv1alpha1.Ignore)
			if err := seedClient.Patch(ctx, managedResource, managedResourcePatch); err != nil {
				return fmt.Errorf("failed to unignore ManagedResource for Prometheus for cluster %s: %w", cluster, err)
			}
		}

		return nil
	}

	getPrometheusWithPatch := func(ctx context.Context, namespace string) (*monitoringv1.Prometheus, client.Patch, error) {
		prometheus := &monitoringv1.Prometheus{}
		if err := seedClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "shoot"}, prometheus); err != nil {
			return nil, nil, err
		}

		return prometheus, client.MergeFrom(prometheus.DeepCopy()), nil
	}

	needsInitContainer := func(prometheus *monitoringv1.Prometheus) bool {
		for _, initContainer := range prometheus.Spec.InitContainers {
			if initContainer.Name == "cleanup-obsolete-folder" {
				return false
			}
		}

		return true
	}

	waitUntilCleanedUp := func(ctx context.Context, log logr.Logger, namespace string) error {
		return retry.UntilTimeout(ctx, 10*time.Second, 10*time.Minute, func(ctx context.Context) (done bool, err error) {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "prometheus-shoot-0"}}
			if err := seedClient.Get(ctx, client.ObjectKeyFromObject(pod), pod); err != nil {
				log.Error(err, "Failed to get prometheus-shoot-0 pod", "namespace", namespace)
				return retry.MinorError(err)
			}

			var hasInitContainer bool
			for _, initContainer := range pod.Spec.InitContainers {
				if initContainer.Name == "cleanup-obsolete-folder" {
					hasInitContainer = true
					break
				}
			}

			if !hasInitContainer {
				err := fmt.Errorf("prometheus-shoot-0 pod does not have the cleanup-obsolete-folder init container")
				log.Error(err, "Pod prometheus-shoot-0 is not cleaned up", "namespace", namespace)
				return retry.MinorError(err)
			}

			if err := health.CheckPod(pod); err != nil {
				log.Error(err, "Pod prometheus-shoot-0 is not healthy", "namespace", namespace)
				return retry.MinorError(err)
			}

			return retry.Ok()
		})
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

	log.Info("Clean up obsolete Prometheus folders")

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
		return fmt.Errorf("failed to list clusters while cleaning up Prometheus obsolete folders: %w", err)
	}

	for _, cluster := range clusterList.Items {
		tasks = append(tasks, func(ctx context.Context) error {
			skip, err := shouldSkipCluster(ctx, log, &cluster)
			if err != nil {
				return err
			}

			if skip {
				log.Info("Skip cleanup for cluster", "cluster", cluster.Name)
				return nil
			}

			if err := unignoreManagedResource(ctx, log, cluster.Name); err != nil {
				return err
			}

			managedResource, managedResourcePatch, err := getManagedResourceWithPatch(ctx, cluster.Name)
			if err != nil {
				if apierrors.IsNotFound(err) {
					log.Info("ManagedResource for Prometheus not found, skipping cleanup", "cluster", cluster.Name)
					return nil
				}
				return fmt.Errorf("failed to get ManagedResource for Prometheus for cluster %s: %w", cluster.Name, err)
			}

			prometheus, prometheusPatch, err := getPrometheusWithPatch(ctx, cluster.Name)
			if err != nil {
				if apierrors.IsNotFound(err) {
					log.Info("Prometheus resource not found, skipping cleanup", "cluster", cluster.Name)
					return nil
				}
				return fmt.Errorf("failed to get Prometheus resource for cluster %s: %w", cluster.Name, err)
			}

			if value, ok := prometheus.Annotations[resourcesv1alpha1.PrometheusObsoleteFolderCleanedUp]; ok && value == "true" {
				// migration already done, nothing to do
				return nil
			}

			// ignore the managed resource temporarily to prevent it from reverting the Prometheus patches
			managedResource.Annotations[resourcesv1alpha1.Ignore] = "true"
			if err := seedClient.Patch(ctx, managedResource, managedResourcePatch); err != nil {
				return fmt.Errorf("failed to ignore ManagedResource for Prometheus for cluster %s: %w", cluster.Name, err)
			}

			log.Info("Clean up obsolete Prometheus folders", "cluster", cluster.Name)

			if needsInitContainer(prometheus) {
				prometheus.Spec.InitContainers = append(prometheus.Spec.InitContainers, corev1.Container{
					Name:            "cleanup-obsolete-folder",
					Image:           *prometheus.Spec.Image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         []string{"sh", "-c", "rm -rf /prometheus/prometheus-; rm -rf /prometheus/prometheus-db/prometheus-"},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: ptr.To(false),
					},
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "prometheus-db",
						MountPath: "/prometheus",
					}},
				})
			}

			prometheus.Spec.Replicas = ptr.To(int32(1))
			if err := seedClient.Patch(ctx, prometheus, prometheusPatch); err != nil {
				if err := unignoreManagedResource(ctx, log, cluster.Name); err != nil {
					log.Error(err, "Failed to unignore ManagedResource for Prometheus after error", "cluster", cluster.Name)
				}
				return fmt.Errorf("failed to patch Prometheus resource for cluster %s: %w", cluster.Name, err)
			}

			if err := waitUntilCleanedUp(ctx, log, cluster.Name); err != nil {
				if err := unignoreManagedResource(ctx, log, cluster.Name); err != nil {
					log.Error(err, "Failed to unignore ManagedResource for Prometheus after error", "cluster", cluster.Name)
				}
				return fmt.Errorf("failed to wait until Prometheus statefulset for cluster %s is cleaned up: %w", cluster.Name, err)
			}

			prometheus, prometheusPatch, err = getPrometheusWithPatch(ctx, cluster.Name)
			if err != nil {
				if err := unignoreManagedResource(ctx, log, cluster.Name); err != nil {
					log.Error(err, "Failed to unignore ManagedResource for Prometheus after error", "cluster", cluster.Name)
				}
				return fmt.Errorf("failed to get Prometheus resource after cleanup for cluster %s: %w", cluster.Name, err)
			}

			prometheus.Annotations[resourcesv1alpha1.PrometheusObsoleteFolderCleanedUp] = "true"
			if err := seedClient.Patch(ctx, prometheus, prometheusPatch); err != nil {
				if err := unignoreManagedResource(ctx, log, cluster.Name); err != nil {
					log.Error(err, "Failed to unignore ManagedResource for Prometheus after error", "cluster", cluster.Name)
				}
				return fmt.Errorf("failed to mark Prometheus resource as cleaned up for cluster %s: %w", cluster.Name, err)
			}

			return unignoreManagedResource(ctx, log, cluster.Name)
		})
	}

	return flow.Parallel(tasks...)(ctx)
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
