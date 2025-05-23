// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

func (k *kubeAPIServer) emptyHorizontalPodAutoscaler() *autoscalingv2.HorizontalPodAutoscaler {
	return &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k.values.NamePrefix + v1beta1constants.DeploymentNameKubeAPIServer,
			Namespace: k.namespace,
		},
	}
}

func (k *kubeAPIServer) reconcileHorizontalPodAutoscaler(ctx context.Context, hpa *autoscalingv2.HorizontalPodAutoscaler, deployment *appsv1.Deployment) error {
	if k.values.Autoscaling.Replicas == nil ||
		*k.values.Autoscaling.Replicas == 0 {
		return kubernetesutils.DeleteObject(ctx, k.client.Client(), hpa)
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client.Client(), hpa, func() error {
		minReplicas := k.values.Autoscaling.MinReplicas
		if k.values.Autoscaling.ScaleDownDisabled && hpa.Spec.MinReplicas != nil {
			// If scale-down is disabled and the HPA resource exists and HPA's spec.minReplicas is not nil,
			// then minReplicas is max(spec.minReplicas, status.desiredReplicas).
			// When scale-down is disabled, this allows operators to specify a custom value for HPA spec.minReplicas
			// and this value not to be reverted by gardenlet.
			minReplicas = max(*hpa.Spec.MinReplicas, hpa.Status.DesiredReplicas)
		}

		metav1.SetMetaDataLabel(&hpa.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigType, resourcesv1alpha1.HighAvailabilityConfigTypeServer)
		hpa.Spec = autoscalingv2.HorizontalPodAutoscalerSpec{
			MinReplicas: &minReplicas,
			MaxReplicas: k.values.Autoscaling.MaxReplicas,
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
				Name:       deployment.Name,
			},
			Metrics: []autoscalingv2.MetricSpec{
				{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: corev1.ResourceCPU,
						Target: autoscalingv2.MetricTarget{
							Type: autoscalingv2.AverageValueMetricType,
							// The chosen value of 6 CPU is aligned with the average value for memory - 24G. Preserve the cpu:memory ratio of 1:4.
							AverageValue: ptr.To(resource.MustParse("6")),
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
							AverageValue: ptr.To(resource.MustParse("24G")),
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
		}

		return nil
	})
	return err
}
