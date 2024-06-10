// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/component-base/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	"github.com/gardener/gardener/imagevector"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenletutils "github.com/gardener/gardener/pkg/utils/gardener/gardenlet"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

func (g *garden) runMigrations(ctx context.Context, log logr.Logger, _ cluster.Cluster) error {
	log.Info("Updating required Gardener-Resource-Manager deployments for managed resource compression support")
	if err := updateGardenerResourceManager(ctx, log, g.mgr.GetClient()); err != nil {
		return err
	}

	log.Info("Migrating deprecated failure-domain.beta.kubernetes.io labels to topology.kubernetes.io")
	if err := migrateDeprecatedTopologyLabels(ctx, log, g.mgr.GetClient(), g.mgr.GetConfig()); err != nil {
		return err
	}

	log.Info("Creating operating system config hash migration secret")
	if err := createOSCHashMigrationSecret(ctx, g.mgr.GetClient()); err != nil {
		return err
	}
	return nil
}

// TODO(timuthy): Remove after v1.97 was released.
// updateGardenerResourceManager updates all GRM deployments in a seed cluster,
// in order to roll out the Brotli compression support added in Gardener v1.96.
func updateGardenerResourceManager(ctx context.Context, log logr.Logger, cl client.Client) error {
	image, err := imagevector.ImageVector().FindImage(imagevector.ImageNameGardenerResourceManager)
	if err != nil {
		return err
	}
	image.WithOptionalTag(version.Get().GitVersion)

	grmDeployments := &appsv1.DeploymentList{}
	if err := cl.List(ctx, grmDeployments, client.MatchingLabels(map[string]string{v1beta1constants.LabelApp: v1beta1constants.DeploymentNameGardenerResourceManager})); err != nil {
		return err
	}

	seedIsGarden, err := gardenletutils.SeedIsGarden(ctx, cl)
	if err != nil {
		return err
	}

	fns := make([]flow.TaskFn, 0, len(grmDeployments.Items))
	for _, grmDeployment := range grmDeployments.Items {
		for j, container := range grmDeployment.Spec.Template.Spec.Containers {
			if container.Name == v1beta1constants.DeploymentNameGardenerResourceManager ||
				grmDeployment.Spec.Template.Spec.Containers[j].Image == image.String() {
				continue
			}

			// Don't patch GRM in Runtime Garden cluster since it is usually not under the control of gardenlet (rather gardener-operator),
			// and already updated to the right version, before Gardenlet is rolled out.
			if grmDeployment.Namespace == v1beta1constants.GardenNamespace && seedIsGarden {
				continue
			}

			fns = append(fns, func(ctx context.Context) error {
				patch := client.StrategicMergeFrom(grmDeployment.DeepCopy())
				grmDeployment.Spec.Template.Spec.Containers[j].Image = image.String()

				log.Info("Updating Gardener-Resource-Manager", "deployment", client.ObjectKeyFromObject(&grmDeployment))
				return cl.Patch(ctx, &grmDeployment, patch)
			})
			break
		}
	}

	return flow.Parallel(fns...)(ctx)
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
