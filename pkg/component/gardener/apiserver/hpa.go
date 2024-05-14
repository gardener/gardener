// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component/apiserver"
	"github.com/gardener/gardener/pkg/utils"
)

func (g *gardenerAPIServer) horizontalPodAutoscaler() *autoscalingv2.HorizontalPodAutoscaler {
	if g.values.Autoscaling.Mode != apiserver.AutoscalingModeVPAAndHPA {
		return nil
	}

	return g.horizontalPodAutoscalerInVPAAndHPAMode()
}

func (g *gardenerAPIServer) horizontalPodAutoscalerInVPAAndHPAMode() *autoscalingv2.HorizontalPodAutoscaler {
	// The chosen value is 6 CPU: 1 CPU less than the VPA's maxAllowed 7 CPU in VPAAndHPA mode to have a headroom for the horizontal scaling.
	hpaTargetAverageValueCPU := resource.MustParse("6")
	// The chosen value is 24G: 4G less than the VPA's maxAllowed 28G in VPAAndHPA mode to have a headroom for the horizontal scaling.
	hpaTargetAverageValueMemory := resource.MustParse("24G")

	return &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName,
			Namespace: g.namespace,
			Labels:    utils.MergeStringMaps(GetLabels(), map[string]string{resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeServer}),
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			MinReplicas: ptr.To[int32](g.values.Autoscaling.MinReplicas),
			MaxReplicas: g.values.Autoscaling.MaxReplicas,
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
							Type:         autoscalingv2.AverageValueMetricType,
							AverageValue: &hpaTargetAverageValueCPU,
						},
					},
				},
				{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: corev1.ResourceMemory,
						Target: autoscalingv2.MetricTarget{
							Type:         autoscalingv2.AverageValueMetricType,
							AverageValue: &hpaTargetAverageValueMemory,
						},
					},
				},
			},
			Behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
				ScaleUp: &autoscalingv2.HPAScalingRules{
					StabilizationWindowSeconds: ptr.To[int32](60),
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
					StabilizationWindowSeconds: ptr.To[int32](1800),
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
}
