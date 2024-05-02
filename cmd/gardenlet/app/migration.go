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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils/flow"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

func (g *garden) runMigrations(ctx context.Context, log logr.Logger, _ cluster.Cluster) error {
	log.Info("Migrating deprecated failure-domain.beta.kubernetes.io labels to topology.kubernetes.io")
	if err := migrateDeprecatedTopologyLabels(ctx, log, g.mgr.GetClient(), g.mgr.GetConfig()); err != nil {
		return err
	}

	log.Info("Deleting orphaned vali VPAs which used to be managed by the HVPA controller")
	return deleteOrphanedValiVPAs(ctx, log, g.mgr.GetClient())
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

// TODO(plkokanov): Remove this code after gardener v1.96 has been released.
// The refactoring done in https://github.com/gardener/gardener/pull/8061 incorrectly changed
// the label used to select vali vpas by the corresponding vali hvpa from `role: vali-vpa` to
// `role: valivpa`. This caused the hvpa controller to orphan the existing vpa objects with
// the `role: vali-vpa` label and create new ones.
// deleteOrphanedValiVPAs deletes the orphaned vali vpas with the `role vali-vpa` label.
func deleteOrphanedValiVPAs(ctx context.Context, log logr.Logger, c client.Client) error {
	var (
		orphanedValiVPALabel = "vali-vpa"
		orphanedVPAList      = &vpaautoscalingv1.VerticalPodAutoscalerList{}
		vpaCRDName           = "verticalpodautoscalers.autoscaling.k8s.io"
		vpaCRD               = &apiextensionsv1.CustomResourceDefinition{}
	)

	if err := c.Get(ctx, types.NamespacedName{Name: vpaCRDName}, vpaCRD); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Could not get required CRD, no need to delete orphaned vali VPA resources", "crd", vpaCRDName)
			return nil
		}
		return fmt.Errorf("could not get %q CRD: %w", vpaCRDName, err)
	}

	if err := c.List(ctx, orphanedVPAList, client.MatchingLabels{v1beta1constants.LabelRole: orphanedValiVPALabel}); err != nil {
		return fmt.Errorf("could not list orphaned vali VPAs with label '%s: %s': %w", v1beta1constants.LabelRole, orphanedValiVPALabel, err)
	}

	fns := make([]flow.TaskFn, 0, len(orphanedVPAList.Items))
	for _, obj := range orphanedVPAList.Items {
		fns = append(fns, func(ctx context.Context) error {
			log.Info("Deleting orphaned vali VPA", "vpa", client.ObjectKeyFromObject(&obj))
			if err := client.IgnoreNotFound(c.Delete(ctx, &obj)); err != nil {
				return fmt.Errorf("could not delete orphaned vali VPA %q: %w", client.ObjectKeyFromObject(&obj).String(), err)
			}
			return nil
		})
	}

	return flow.Parallel(fns...)(ctx)
}
