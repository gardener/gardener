// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package migration

import (
	"context"
	"fmt"
	"math"

	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

const (
	// vpasChunkSize defines the number of VPA resources within a parallel processed chunk.
	vpasChunkSize = 100
)

// MigrateVPAEmptyPatch performs an empty patch updates to VerticalPodAutoscaler resources,
// that are elighible to adopt the InPlaceOrRecreate update mode, but are filtered out, because of
// the Resource Manager's alwaysUpdate=false configuration.
// TODO(vitanovs): Remove the migration once the VPAInPlaceUpdates feature gates promoted to GA.
func MigrateVPAEmptyPatch(ctx context.Context, mgr manager.Manager, log logr.Logger) error {
	log.Info("Migrating VerticalPodAutoscalers")

	vpaList := vpaautoscalingv1.VerticalPodAutoscalerList{}
	if err := mgr.GetClient().List(ctx, &vpaList); err != nil {
		if meta.IsNoMatchError(err) {
			log.Info("Resources kind not found, skipping migration", "kind", "VerticalPodAutoscaler")
			return nil
		}
		return fmt.Errorf("failed listing VerticalPodAutoscaler resources: %w", err)
	}

	for _, vpa := range vpaList.Items {
		vpaHasSkipLabel := metav1.HasLabel(vpa.ObjectMeta, resourcesv1alpha1.VPAInPlaceUpdatesSkip)
		vpaHasMutatedLabel := metav1.HasLabel(vpa.ObjectMeta, resourcesv1alpha1.VPAInPlaceUpdatesMutated)
		if vpaHasSkipLabel || vpaHasMutatedLabel {
			continue
		}

		if vpa.Namespace == metav1.NamespaceSystem || vpa.Namespace == v1beta1constants.KubernetesDashboardNamespace {
			continue
		}

		updateMode := ptr.Deref(vpa.Spec.UpdatePolicy.UpdateMode, vpaautoscalingv1.UpdateModeRecreate)
		if updateMode != vpaautoscalingv1.UpdateModeAuto && updateMode != vpaautoscalingv1.UpdateModeRecreate {
			continue
		}

		vpaKey := client.ObjectKeyFromObject(&vpa)
		log.Info("Updating VerticalPodAutoscaler resource", "vpa", vpaKey)

		patch := client.RawPatch(types.MergePatchType, []byte("{}"))
		if err := mgr.GetClient().Patch(ctx, &vpa, patch); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("Resource not found, skipping migration", "vpa", vpaKey)
				continue
			}
			return fmt.Errorf("failed updating VerticalPodAutoscaler '%s': %w", vpaKey, err)
		}
	}
	return nil
}

// MigrateVPAUpdateModeToRecreate applies a patch to VerticalPodAutoscaler resources that
// sets their update modes to Recreate, in order to undo the change applied by a GRM mutation webhook.
// TODO(vitanovs): Remove the migration once the VPAInPlaceUpdates feature gates promoted to GA.
func MigrateVPAUpdateModeToRecreate(ctx context.Context, mgr manager.Manager, log logr.Logger) error {
	log.Info("Migrating VerticalPodAutoscalers to update mode Recreate")

	vpaList := vpaautoscalingv1.VerticalPodAutoscalerList{}
	if err := mgr.GetClient().List(ctx, &vpaList); err != nil {
		if meta.IsNoMatchError(err) {
			log.Info("Resources kind not found, skipping update mode migration", "kind", "VerticalPodAutoscaler")
			return nil
		}
		return fmt.Errorf("failed listing VerticalPodAutoscaler resources: %w", err)
	}

	var tasks []flow.TaskFn
	for _, vpa := range vpaList.Items {
		task := func(ctx context.Context) error {
			vpaHasSkipLabel := metav1.HasLabel(vpa.ObjectMeta, resourcesv1alpha1.VPAInPlaceUpdatesSkip)
			vpaHasMutatedLabel := metav1.HasLabel(vpa.ObjectMeta, resourcesv1alpha1.VPAInPlaceUpdatesMutated)
			if vpaHasSkipLabel || !vpaHasMutatedLabel {
				return nil
			}

			if vpa.Namespace == metav1.NamespaceSystem || vpa.Namespace == v1beta1constants.KubernetesDashboardNamespace {
				return nil
			}

			vpaKey := client.ObjectKeyFromObject(&vpa)
			log.Info("Migrating VerticalPodAutoscaler resource update mode to Recreate", "vpa", vpaKey)

			patch := client.MergeFrom(vpa.DeepCopy())

			vpa.Spec.UpdatePolicy.UpdateMode = ptr.To(vpaautoscalingv1.UpdateModeRecreate)
			delete(vpa.Labels, resourcesv1alpha1.VPAInPlaceUpdatesMutated)

			if err := mgr.GetClient().Patch(ctx, &vpa, patch); err != nil {
				if apierrors.IsNotFound(err) {
					log.Info("Resource not found, skipping update mode migration", "vpa", vpaKey)
				}
				return fmt.Errorf("failed migrating VerticalPodAutoscaler '%s' update mode: %w", vpaKey, err)
			}
			return nil
		}
		tasks = append(tasks, task)
	}

	workersCount := int(math.Ceil(float64(len(vpaList.Items) / vpasChunkSize)))
	return flow.ParallelN(workersCount, tasks...)(ctx)
}
