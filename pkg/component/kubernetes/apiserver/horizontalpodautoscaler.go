// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	hpaTargetAverageUtilizationCPU    int32 = 80
	hpaTargetAverageUtilizationMemory int32 = 80
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
	if k.values.Autoscaling.HVPAEnabled ||
		k.values.Autoscaling.Replicas == nil ||
		*k.values.Autoscaling.Replicas == 0 {
		return kubernetesutils.DeleteObject(ctx, k.client.Client(), hpa)
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client.Client(), hpa, func() error {
		hpa.Spec = autoscalingv2.HorizontalPodAutoscalerSpec{
			MinReplicas: &k.values.Autoscaling.MinReplicas,
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
							Type:               autoscalingv2.UtilizationMetricType,
							AverageUtilization: ptr.To(hpaTargetAverageUtilizationCPU),
						},
					},
				},
				{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: corev1.ResourceMemory,
						Target: autoscalingv2.MetricTarget{
							Type:               autoscalingv2.UtilizationMetricType,
							AverageUtilization: ptr.To(hpaTargetAverageUtilizationMemory),
						},
					},
				},
			},
		}

		return nil
	})

	return err
}
