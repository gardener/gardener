// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/controllerutils"
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
			UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
		}
		vpa.Spec.ResourcePolicy = &vpaautoscalingv1.PodResourcePolicy{
			ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
				ContainerName: name,
				MinAllowed: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("200Mi"),
				},
			}},
		}
		return nil
	})
	return err
}

// DeleteVPAForGardenerComponent deletes a VPA for a Gardener component.
func DeleteVPAForGardenerComponent(ctx context.Context, c client.Client, name, namespace string) error {
	return client.IgnoreNotFound(c.Delete(ctx, emptyVPA(name, namespace)))
}

func emptyVPA(name, namespace string) *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: name + "-vpa", Namespace: namespace}}
}
