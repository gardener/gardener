// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/flow"
)

// ReconcileVPAForGardenerComponent deploys a VPA for a Gardener component.
func ReconcileVPAForGardenerComponent(ctx context.Context, c client.Client, name, namespace string) error {
	vpa := emptyVPA(name, namespace)

	_, err := controllerutils.CreateOrGetAndMergePatch(ctx, c, vpa, func() error {
		vpa.Spec.TargetRef = &autoscalingv1.CrossVersionObjectReference{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
			Name:       name,
		}
		vpa.Spec.UpdatePolicy = &vpaautoscalingv1.PodUpdatePolicy{
			UpdateMode: new(vpaautoscalingv1.UpdateModeInPlaceOrRecreate),
		}
		vpa.Spec.ResourcePolicy = &vpaautoscalingv1.PodResourcePolicy{
			ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
				{
					ContainerName: name,
					MinAllowed: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("200Mi"),
					},
				},
				{
					ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
					Mode:          new(vpaautoscalingv1.ContainerScalingModeOff),
				},
			},
		}
		return nil
	})
	return err
}

// DeleteVPAForGardenerComponent deletes a VPA for a Gardener component.
func DeleteVPAForGardenerComponent(ctx context.Context, c client.Client, name, namespace string) error {
	if err := c.Delete(ctx, emptyVPA(name, namespace)); err != nil {
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return nil
		}
		return err
	}
	return nil
}

func emptyVPA(name, namespace string) *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: name + "-vpa", Namespace: namespace}}
}

// vpaInPlaceUpdatesMutatedLabel is the label that was set on VerticalPodAutoscalers by the removed
// vpa-in-place-updates webhook of gardener-resource-manager.
const vpaInPlaceUpdatesMutatedLabel = "vpa-in-place-updates.resources.gardener.cloud/mutated"

// RemoveVPAInPlaceUpdatesMutatedLabel removes the `vpa-in-place-updates.resources.gardener.cloud/mutated` label from
// all VerticalPodAutoscalers in the cluster.
// TODO(DobromirNPeev): Remove this function once the migration has been rolled out to all clusters, i.e. after a
// release containing it has been published and clusters have been upgraded past it.
func RemoveVPAInPlaceUpdatesMutatedLabel(ctx context.Context, reader client.Reader, c client.Client) error {
	vpaList := &vpaautoscalingv1.VerticalPodAutoscalerList{}
	if err := reader.List(ctx, vpaList, client.HasLabels{vpaInPlaceUpdatesMutatedLabel}); err != nil {
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
			if err := c.Patch(ctx, &vpa, patch); client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("failed removing label %q from VerticalPodAutoscaler %s: %w", vpaInPlaceUpdatesMutatedLabel, client.ObjectKeyFromObject(&vpa), err)
			}
			return nil
		})
	}

	return flow.ParallelN(20, taskFns...)(ctx)
}
