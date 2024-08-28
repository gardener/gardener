// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/component/apiserver"
)

func (g *gardenerAPIServer) verticalPodAutoscaler() *vpaautoscalingv1.VerticalPodAutoscaler {
	switch g.values.Autoscaling.Mode {
	case apiserver.AutoscalingModeHVPA:
		return nil
	case apiserver.AutoscalingModeVPAAndHPA:
		return g.verticalPodAutoscalerInVPAAndHPAMode()
	default:
		return g.verticalPodAutoscalerInBaselineMode()
	}
}

func (g *gardenerAPIServer) verticalPodAutoscalerInBaselineMode() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName + "-vpa",
			Namespace: g.namespace,
			Labels:    GetLabels(),
		},
		Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
				Name:       DeploymentName,
			},
			UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
			},
			ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
				ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
					{
						ContainerName: containerName,
						MinAllowed: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
						MaxAllowed: g.values.Autoscaling.VPAMaxAllowed,
					},
				},
			},
		},
	}
}

func (g *gardenerAPIServer) verticalPodAutoscalerInVPAAndHPAMode() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName + "-vpa",
			Namespace: g.namespace,
			Labels:    GetLabels(),
		},
		Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
				Name:       DeploymentName,
			},
			UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
			},
			ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
				ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
					{
						ContainerName: containerName,
						MinAllowed: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("200M"),
						},
						MaxAllowed:       g.values.Autoscaling.VPAMaxAllowed,
						ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
					},
				},
			},
		},
	}
}
