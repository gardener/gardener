// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
)

func (p *prometheus) vpa() *vpaautoscalingv1.VerticalPodAutoscaler {
	obj := &vpaautoscalingv1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.name(),
			Namespace: p.namespace,
			Labels: utils.MergeStringMaps(p.getLabels(), map[string]string{
				v1beta1constants.LabelObservabilityApplication: p.name(),
			}),
		},
		Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				APIVersion: monitoringv1.SchemeGroupVersion.String(),
				Kind:       "Prometheus",
				Name:       p.values.Name,
			},
			UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
			},
			ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
				ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
					{
						ContainerName: "prometheus",
						MinAllowed: ptr.Deref(p.values.VPAMinAllowed, corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("100M"),
						}),
						ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
					},
					{
						ContainerName: "config-reloader",
						Mode:          ptr.To(vpaautoscalingv1.ContainerScalingModeOff),
					},
				},
			},
		},
	}

	if p.values.Cortex != nil {
		obj.Spec.ResourcePolicy.ContainerPolicies = append(obj.Spec.ResourcePolicy.ContainerPolicies, vpaautoscalingv1.ContainerResourcePolicy{
			ContainerName: containerNameCortex,
			Mode:          ptr.To(vpaautoscalingv1.ContainerScalingModeOff),
		})
	}

	return obj
}
