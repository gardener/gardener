// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenermetricscollector

import (
	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
)

const collectorContainerName = "otc-container"

func (g *gardenerMetricsCollector) verticalPodAutoscaler() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      openTelemetryCollectorName,
			Namespace: g.namespace,
			Labels:    getLabels(),
		},
		Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				APIVersion: otelv1beta1.GroupVersion.String(),
				Kind:       "OpenTelemetryCollector",
				Name:       openTelemetryCollectorName,
			},
			UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: new(vpaautoscalingv1.UpdateModeInPlaceOrRecreate),
			},
			ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
				ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
					{
						ContainerName: collectorContainerName,
						MinAllowed: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
						ControlledValues: new(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
					},
					{
						ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
						Mode:          new(vpaautoscalingv1.ContainerScalingModeOff),
					},
				},
			},
		},
	}
}
