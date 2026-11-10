// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package migration

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/utils/flow"
)

// vpaInPlaceUpdatesMutatedLabel is the label that was set on VerticalPodAutoscalers by the removed
// vpa-in-place-updates webhook of gardener-resource-manager.
const vpaInPlaceUpdatesMutatedLabel = "vpa-in-place-updates.resources.gardener.cloud/mutated"

// vpaParallelWorkersCount is the maximum number of VerticalPodAutoscalers patched in parallel.
const vpaParallelWorkersCount = 20

// RemoveVPAInPlaceUpdatesMutatedLabel removes the `vpa-in-place-updates.resources.gardener.cloud/mutated` label from
// all VerticalPodAutoscalers in the cluster.
//
// TODO(DobromirNPeev): Remove this function after Gardener v1.158 has been released.
func RemoveVPAInPlaceUpdatesMutatedLabel(ctx context.Context, c client.Client, log logr.Logger) error {
	vpaList := &vpaautoscalingv1.VerticalPodAutoscalerList{}
	if err := c.List(ctx, vpaList, client.HasLabels{vpaInPlaceUpdatesMutatedLabel}); err != nil {
		if meta.IsNoMatchError(err) {
			return nil
		}
		return fmt.Errorf("failed listing VerticalPodAutoscalers: %w", err)
	}

	taskFns := make([]flow.TaskFn, 0, len(vpaList.Items))
	for _, vpa := range vpaList.Items {
		taskFns = append(taskFns, func(ctx context.Context) error {
			patch := client.MergeFrom(vpa.DeepCopy())
			delete(vpa.Labels, vpaInPlaceUpdatesMutatedLabel)
			log.Info("Removing label from VerticalPodAutoscaler", "label", vpaInPlaceUpdatesMutatedLabel, "vpa", client.ObjectKeyFromObject(&vpa))
			if err := c.Patch(ctx, &vpa, patch); client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("failed removing label %q from VerticalPodAutoscaler %s: %w", vpaInPlaceUpdatesMutatedLabel, client.ObjectKeyFromObject(&vpa), err)
			}
			return nil
		})
	}

	return flow.ParallelN(vpaParallelWorkersCount, taskFns...)(ctx)
}
