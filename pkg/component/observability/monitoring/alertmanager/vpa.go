// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package alertmanager

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
)

func (a *alertManager) vpa() *vpaautoscalingv1.VerticalPodAutoscaler {
	updateMode, controlledValuesRequestsOnly, containerScalingModeOff := vpaautoscalingv1.UpdateModeAuto, vpaautoscalingv1.ContainerControlledValuesRequestsOnly, vpaautoscalingv1.ContainerScalingModeOff

	return &vpaautoscalingv1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      a.name(),
			Namespace: a.namespace,
			Labels: utils.MergeStringMaps(a.getLabels(), map[string]string{
				v1beta1constants.LabelObservabilityApplication: a.name(),
			}),
		},
		Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				APIVersion: monitoringv1.SchemeGroupVersion.String(),
				Kind:       "Alertmanager",
				Name:       a.values.Name,
			},
			UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: &updateMode,
			},
			ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
				ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
					{
						ContainerName: "alertmanager",
						MinAllowed: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("20Mi"),
						},
						MaxAllowed: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("200Mi"),
						},
						ControlledValues:    &controlledValuesRequestsOnly,
						ControlledResources: &[]corev1.ResourceName{corev1.ResourceMemory},
					},
					{
						ContainerName: "config-reloader",
						Mode:          &containerScalingModeOff,
					},
				},
			},
		},
	}
}
