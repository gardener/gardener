// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gardenerapiserver

import (
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

func (g *gardenerAPIServer) hvpa() *hvpav1alpha1.Hvpa {
	if !g.values.Autoscaling.HVPAEnabled {
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
			Replicas: pointer.Int32(1),
			Hpa: hvpav1alpha1.HpaSpec{
				Selector: &metav1.LabelSelector{MatchLabels: hpaLabels},
				Deploy:   true,
				ScaleUp: hvpav1alpha1.ScaleType{
					UpdatePolicy: hvpav1alpha1.UpdatePolicy{
						UpdateMode: pointer.String(hvpav1alpha1.UpdateModeAuto),
					},
				},
				ScaleDown: hvpav1alpha1.ScaleType{
					UpdatePolicy: hvpav1alpha1.UpdatePolicy{
						UpdateMode: pointer.String(hvpav1alpha1.UpdateModeAuto),
					},
				},
				Template: hvpav1alpha1.HpaTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: hpaLabels,
					},
					Spec: hvpav1alpha1.HpaTemplateSpec{
						MinReplicas: pointer.Int32(replicas),
						MaxReplicas: maxReplicas,
						Metrics: []autoscalingv2beta1.MetricSpec{
							{
								Type: autoscalingv2beta1.ResourceMetricSourceType,
								Resource: &autoscalingv2beta1.ResourceMetricSource{
									Name:                     corev1.ResourceCPU,
									TargetAverageUtilization: pointer.Int32(80),
								},
							},
							{
								Type: autoscalingv2beta1.ResourceMetricSourceType,
								Resource: &autoscalingv2beta1.ResourceMetricSource{
									Name:                     corev1.ResourceMemory,
									TargetAverageUtilization: pointer.Int32(80),
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
						UpdateMode: pointer.String(hvpav1alpha1.UpdateModeAuto),
					},
					StabilizationDuration: pointer.String("3m"),
					MinChange: hvpav1alpha1.ScaleParams{
						CPU: hvpav1alpha1.ChangeParams{
							Value:      pointer.String("300m"),
							Percentage: pointer.Int32(80),
						},
						Memory: hvpav1alpha1.ChangeParams{
							Value:      pointer.String("200M"),
							Percentage: pointer.Int32(80),
						},
					},
				},
				ScaleDown: hvpav1alpha1.ScaleType{
					UpdatePolicy: hvpav1alpha1.UpdatePolicy{
						UpdateMode: pointer.String(hvpav1alpha1.UpdateModeAuto),
					},
					StabilizationDuration: pointer.String("15m"),
					MinChange: hvpav1alpha1.ScaleParams{
						CPU: hvpav1alpha1.ChangeParams{
							Value:      pointer.String("600m"),
							Percentage: pointer.Int32(80),
						},
						Memory: hvpav1alpha1.ChangeParams{
							Value:      pointer.String("600M"),
							Percentage: pointer.Int32(80),
						},
					},
				},
				LimitsRequestsGapScaleParams: hvpav1alpha1.ScaleParams{
					CPU: hvpav1alpha1.ChangeParams{
						Value:      pointer.String("1"),
						Percentage: pointer.Int32(70),
					},
					Memory: hvpav1alpha1.ChangeParams{
						Value:      pointer.String("1G"),
						Percentage: pointer.Int32(70),
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
								MaxAllowed: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("4"),
									corev1.ResourceMemory: resource.MustParse("25G"),
								},
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
