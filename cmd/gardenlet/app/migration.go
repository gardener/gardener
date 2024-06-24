// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/component/nodemanagement/dependencywatchdog"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

func (g *garden) runMigrations(ctx context.Context, log logr.Logger, gardenCluster cluster.Cluster, seedName string) error {
	log.Info("Migrating deprecated failure-domain.beta.kubernetes.io labels to topology.kubernetes.io")
	if err := migrateDeprecatedTopologyLabels(ctx, log, g.mgr.GetClient(), g.mgr.GetConfig()); err != nil {
		return err
	}

	log.Info("Creating operating system config hash migration secret")
	if err := createOSCHashMigrationSecret(ctx, g.mgr.GetClient()); err != nil {
		return err
	}

	log.Info("Cleaning up DWD access for workerless shoots")
	if err := cleanupDWDAccess(ctx, gardenCluster.GetClient(), g.mgr.GetClient(), seedName); err != nil {
		return err
	}

	return nil
}

// TODO: Remove this function when Kubernetes 1.27 support gets dropped.
func migrateDeprecatedTopologyLabels(ctx context.Context, log logr.Logger, seedClient client.Client, restConfig *rest.Config) error {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed creating discovery client: %w", err)
	}

	version, err := discoveryClient.ServerVersion()
	if err != nil {
		return fmt.Errorf("failed reading the server version of seed cluster: %w", err)
	}

	seedVersion, err := semver.NewVersion(version.GitVersion)
	if err != nil {
		return fmt.Errorf("failed parsing server version to semver: %w", err)
	}

	//  PV node affinities were immutable until Kubernetes 1.27, see https://github.com/kubernetes/kubernetes/pull/115391
	if !versionutils.ConstraintK8sGreaterEqual127.Check(seedVersion) {
		return nil
	}

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

// TODO(MichaelEischer): Remove this function after v1.99 has been released
func createOSCHashMigrationSecret(ctx context.Context, seedClient client.Client) error {
	namespaceList := &corev1.NamespaceList{}
	if err := seedClient.List(ctx, namespaceList, client.MatchingLabels(map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot})); err != nil {
		return err
	}

	var tasks []flow.TaskFn
	for _, ns := range namespaceList.Items {
		if ns.DeletionTimestamp != nil || ns.Status.Phase == corev1.NamespaceTerminating {
			continue
		}
		tasks = append(tasks, func(ctx context.Context) error {
			if err := seedClient.Get(ctx, types.NamespacedName{Namespace: ns.Name, Name: operatingsystemconfig.WorkerPoolHashesSecretName}, &corev1.Secret{}); err == nil {
				// nothing to do if secret already exists
				return nil
			} else if client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("could not query worker-pools-operatingsystemconfig-hashes secret in namespace %v: %w", ns.Name, err)
			}

			secret, err := operatingsystemconfig.CreateMigrationSecret(ns.Name)
			if err != nil {
				return fmt.Errorf("failed to serialize worker-pools-operatingsystemconfig-hashes secret for namespace %v: %w", ns.Name, err)
			}

			if err := seedClient.Create(ctx, secret); client.IgnoreAlreadyExists(err) != nil {
				return fmt.Errorf("could not create worker-pools-operatingsystemconfig-hashes secret in namespace %v: %w", ns.Name, err)
			}

			return nil
		})
	}
	return flow.Parallel(tasks...)(ctx)
}

// TODO (shafeeqes): Remove this function in gardener v1.100
func cleanupDWDAccess(ctx context.Context, gardenClient client.Client, seedClient client.Client, seedName string) error {
	shootList := &gardencorev1beta1.ShootList{}
	if err := gardenClient.List(ctx, shootList, client.MatchingFields{core.ShootSeedName: seedName}); err != nil {
		return err
	}

	var taskFns []flow.TaskFn

	for _, shoot := range shootList.Items {
		if !v1beta1helper.IsWorkerless(&shoot) || shoot.DeletionTimestamp != nil {
			continue
		}

		namespace := shoot.Status.TechnicalID
		taskFns = append(taskFns, func(ctx context.Context) error {
			if err := kubernetesutils.DeleteObjects(ctx, seedClient,
				&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: dependencywatchdog.KubeConfigSecretName, Namespace: namespace}},
			); err != nil {
				return fmt.Errorf("failed to delete DWD access secret for namespace %q: %w", namespace, err)
			}

			if err := managedresources.DeleteForShoot(ctx, seedClient, namespace, dependencywatchdog.ManagedResourceName); err != nil {
				return fmt.Errorf("failed to delete DWD managed resource for namespace %q: %w", namespace, err)
			}

			return nil
		})
	}

	return flow.Parallel(taskFns...)(ctx)
}
