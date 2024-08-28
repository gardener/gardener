// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component/apiserver"
	"github.com/gardener/gardener/pkg/utils"
)

func (g *gardenerAPIServer) hvpa() *hvpav1alpha1.Hvpa {
	if g.values.Autoscaling.Mode != apiserver.AutoscalingModeHVPA {
		return nil
	}

	var (
		replicas    int32 = 1
		maxReplicas int32 = 4
		hpaLabels         = map[string]string{"role": "gardener-apiserver-hpa"}
		vpaLabels         = map[string]string{"role": "gardener-apiserver-vpa"}
	)

	return &hvpav1alpha1.Hvpa{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName + "-hvpa",
			Namespace: g.namespace,
			Labels:    utils.MergeStringMaps(GetLabels(), map[string]string{resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeServer}),
		},
		Spec: hvpav1alpha1.HvpaSpec{
			Replicas: ptr.To[int32](1),
			Hpa: hvpav1alpha1.HpaSpec{
				Selector: &metav1.LabelSelector{MatchLabels: hpaLabels},
				Deploy:   true,
				ScaleUp: hvpav1alpha1.ScaleType{
					UpdatePolicy: hvpav1alpha1.UpdatePolicy{
						UpdateMode: ptr.To(hvpav1alpha1.UpdateModeAuto),
					},
				},
				ScaleDown: hvpav1alpha1.ScaleType{
					UpdatePolicy: hvpav1alpha1.UpdatePolicy{
						UpdateMode: ptr.To(hvpav1alpha1.UpdateModeAuto),
					},
				},
				Template: hvpav1alpha1.HpaTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: hpaLabels,
					},
					Spec: hvpav1alpha1.HpaTemplateSpec{
						MinReplicas: ptr.To(replicas),
						MaxReplicas: maxReplicas,
						Metrics: []autoscalingv2beta1.MetricSpec{
							{
								Type: autoscalingv2beta1.ResourceMetricSourceType,
								Resource: &autoscalingv2beta1.ResourceMetricSource{
									Name:                     corev1.ResourceCPU,
									TargetAverageUtilization: ptr.To[int32](80),
								},
							},
							{
								Type: autoscalingv2beta1.ResourceMetricSourceType,
								Resource: &autoscalingv2beta1.ResourceMetricSource{
									Name:                     corev1.ResourceMemory,
									TargetAverageUtilization: ptr.To[int32](80),
								},
							},
						},
					},
				},
			},
			Vpa: hvpav1alpha1.VpaSpec{
				Selector: &metav1.LabelSelector{MatchLabels: vpaLabels},
				Deploy:   true,
				ScaleUp: hvpav1alpha1.ScaleType{
					UpdatePolicy: hvpav1alpha1.UpdatePolicy{
						UpdateMode: ptr.To(hvpav1alpha1.UpdateModeAuto),
					},
					StabilizationDuration: ptr.To("3m"),
					MinChange: hvpav1alpha1.ScaleParams{
						CPU: hvpav1alpha1.ChangeParams{
							Value:      ptr.To("300m"),
							Percentage: ptr.To[int32](80),
						},
						Memory: hvpav1alpha1.ChangeParams{
							Value:      ptr.To("200M"),
							Percentage: ptr.To[int32](80),
						},
					},
				},
				ScaleDown: hvpav1alpha1.ScaleType{
					UpdatePolicy: hvpav1alpha1.UpdatePolicy{
						UpdateMode: ptr.To(hvpav1alpha1.UpdateModeAuto),
					},
					StabilizationDuration: ptr.To("15m"),
					MinChange: hvpav1alpha1.ScaleParams{
						CPU: hvpav1alpha1.ChangeParams{
							Value:      ptr.To("600m"),
							Percentage: ptr.To[int32](80),
						},
						Memory: hvpav1alpha1.ChangeParams{
							Value:      ptr.To("600M"),
							Percentage: ptr.To[int32](80),
						},
					},
				},
				LimitsRequestsGapScaleParams: hvpav1alpha1.ScaleParams{
					CPU: hvpav1alpha1.ChangeParams{
						Value:      ptr.To("1"),
						Percentage: ptr.To[int32](70),
					},
					Memory: hvpav1alpha1.ChangeParams{
						Value:      ptr.To("1G"),
						Percentage: ptr.To[int32](70),
					},
				},
				Template: hvpav1alpha1.VpaTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: vpaLabels,
					},
					Spec: hvpav1alpha1.VpaTemplateSpec{
						ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
							ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
								ContainerName: containerName,
								MinAllowed: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("400M"),
								},
								MaxAllowed: g.values.Autoscaling.VPAMaxAllowed,
							}},
						},
					},
				},
			},
			WeightBasedScalingIntervals: []hvpav1alpha1.WeightBasedScalingInterval{{
				VpaWeight:         hvpav1alpha1.HpaOnly,
				StartReplicaCount: replicas,
				LastReplicaCount:  maxReplicas - 1,
			}},
			TargetRef: &autoscalingv2beta1.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
				Name:       DeploymentName,
			},
		},
	}
}
