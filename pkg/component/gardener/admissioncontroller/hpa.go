// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package admissioncontroller

import (
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

func (a *gardenerAdmissionController) hpa() *autoscalingv2.HorizontalPodAutoscaler {
	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName + "-hpa",
			Namespace: a.namespace,
			Labels:    GetLabels(),
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			MinReplicas: &a.values.Autoscaling.MinReplicas,
			MaxReplicas: a.values.Autoscaling.MaxReplicas,
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
				Name:       DeploymentName,
			},
			Metrics: []autoscalingv2.MetricSpec{
				{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: corev1.ResourceCPU,
						Target: autoscalingv2.MetricTarget{
							Type: autoscalingv2.AverageValueMetricType,
							// The chosen value of 6 CPU is aligned with the average value for memory - 24G. Preserve the cpu:memory ratio of 1:4.
							AverageValue: new(resource.MustParse("6")),
						},
					},
				},
				{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: corev1.ResourceMemory,
						Target: autoscalingv2.MetricTarget{
							Type: autoscalingv2.AverageValueMetricType,
							// The chosen value of 24G is aligned with the average value for cpu - 6 CPU cores. Preserve the cpu:memory ratio of 1:4.
							AverageValue: new(resource.MustParse("24G")),
						},
					},
				},
			},
			Behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
				ScaleUp: &autoscalingv2.HPAScalingRules{
					StabilizationWindowSeconds: new((int32)(60)),
					Policies: []autoscalingv2.HPAScalingPolicy{
						// Allow to upscale 100% of the current number of pods every 1 minute to see whether any upscale recommendation will still hold true after the cluster has settled
						{
							Type:          autoscalingv2.PercentScalingPolicy,
							Value:         100,
							PeriodSeconds: 60,
						},
					},
				},
				ScaleDown: &autoscalingv2.HPAScalingRules{
					StabilizationWindowSeconds: new((int32)(1800)),
					Policies: []autoscalingv2.HPAScalingPolicy{
						// Allow to downscale one pod every 5 minutes to see whether any downscale recommendation will still hold true after the cluster has settled (conservatively)
						{
							Type:          autoscalingv2.PodsScalingPolicy,
							Value:         1,
							PeriodSeconds: 300,
						},
					},
				},
			},
		},
	}

	metav1.SetMetaDataLabel(&hpa.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigType, resourcesv1alpha1.HighAvailabilityConfigTypeServer)

	return hpa
}
