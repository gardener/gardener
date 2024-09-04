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
	"k8s.io/apimachinery/pkg/api/meta"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	utilclient "github.com/gardener/gardener/pkg/utils/kubernetes/client"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

func (g *garden) runMigrations(ctx context.Context, log logr.Logger) error {
	log.Info("Migrating deprecated failure-domain.beta.kubernetes.io labels to topology.kubernetes.io")
	if err := migrateDeprecatedTopologyLabels(ctx, log, g.mgr.GetClient(), g.mgr.GetConfig()); err != nil {
		return err
	}

	log.Info("Deleting stale alertmanager VPAs")
	if err := deleteStaleAlertmanagerVPAs(ctx, log, g.mgr.GetClient()); err != nil {
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

// deleteStaleAlertmanagerVPAs deletes VPAs with name "alertmanager-vpa" in the Seed cluster.
// In https://github.com/gardener/gardener/pull/9257/files#diff-31f2d707167e32b68a064c12fe955a5f2e7d6668e4de445ea9bf6b4e125e6091R95-R103 we forgot to delete
// the VPA related to the old alertmanager deployment (https://github.com/gardener/gardener/pull/9257/files#diff-48abc7fddac745815a412837ac95081265c7ffcd80d5cbf3b2ec1454b2a4068aL160-L175).
//
// TODO(ialidzhikov): Remove this function after v1.106 has been released.
func deleteStaleAlertmanagerVPAs(ctx context.Context, log logr.Logger, seedClient client.Client) error {
	vpas := &vpaautoscalingv1.VerticalPodAutoscalerList{}
	if err := seedClient.List(ctx, vpas); err != nil {
		if meta.IsNoMatchError(err) {
			log.Info("Received a 'no match error' while trying to list VPAs. Will assume that the VPA CRD is not yet installed (for example new Seed creation) and will skip cleaning up stale alertmanager VPAs")
			return nil
		}

		return err
	}

	return utilclient.ApplyToObjects(ctx, vpas, func(ctx context.Context, obj client.Object) error {
		if obj.GetName() == "alertmanager-vpa" {
			if err := kubernetesutils.DeleteObject(ctx, seedClient, obj); err != nil {
				return fmt.Errorf("failed to delete VPA %s: %w", client.ObjectKeyFromObject(obj), err)
			}
		}

		return nil
	})
}
