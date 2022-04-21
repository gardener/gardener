// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubeapiserver

import (
	"context"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
)

func (k *kubeAPIServer) emptyHVPA() *hvpav1alpha1.Hvpa {
	return &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: k.namespace}}
}

func (k *kubeAPIServer) reconcileHVPA(ctx context.Context, hvpa *hvpav1alpha1.Hvpa, deployment *appsv1.Deployment) error {
	if !k.values.Autoscaling.HVPAEnabled ||
		k.values.Autoscaling.Replicas == nil ||
		*k.values.Autoscaling.Replicas == 0 {

		return kutil.DeleteObject(ctx, k.client.Client(), hvpa)
	}

	var (
		hpaLabels           = map[string]string{v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer + "-hpa"}
		vpaLabels           = map[string]string{v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer + "-vpa"}
		updateModeAuto      = hvpav1alpha1.UpdateModeAuto
		scaleDownUpdateMode = updateModeAuto
		containerPolicyOff  = vpaautoscalingv1.ContainerScalingModeOff
		controlledValues    = vpaautoscalingv1.ContainerControlledValuesRequestsOnly
		hpaMetrics          = []autoscalingv2beta1.MetricSpec{
			{
				Type: autoscalingv2beta1.ResourceMetricSourceType,
				Resource: &autoscalingv2beta1.ResourceMetricSource{
					Name:                     corev1.ResourceCPU,
					TargetAverageUtilization: pointer.Int32(hpaTargetAverageUtilizationCPU),
				},
			},
		}
		vpaContainerResourcePolicies = []vpaautoscalingv1.ContainerResourcePolicy{
			{
				ContainerName: ContainerNameKubeAPIServer,
				MinAllowed: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("300m"),
					corev1.ResourceMemory: resource.MustParse("400M"),
				},
				MaxAllowed: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("8"),
					corev1.ResourceMemory: resource.MustParse("25G"),
				},
				ControlledValues: &controlledValues,
			},
			{
				ContainerName:    containerNameVPNSeed,
				Mode:             &containerPolicyOff,
				ControlledValues: &controlledValues,
			},
		}
		weightBasedScalingIntervals = []hvpav1alpha1.WeightBasedScalingInterval{
			{
				VpaWeight:         hvpav1alpha1.VpaOnly,
				StartReplicaCount: k.values.Autoscaling.MaxReplicas,
				LastReplicaCount:  k.values.Autoscaling.MaxReplicas,
			},
		}
	)

	if k.values.Autoscaling.UseMemoryMetricForHvpaHPA {
		hpaMetrics = append(hpaMetrics, autoscalingv2beta1.MetricSpec{
			Type: autoscalingv2beta1.ResourceMetricSourceType,
			Resource: &autoscalingv2beta1.ResourceMetricSource{
				Name:                     corev1.ResourceMemory,
				TargetAverageUtilization: pointer.Int32(hpaTargetAverageUtilizationMemory),
			},
		})
	}

	if k.values.Autoscaling.ScaleDownDisabledForHvpa {
		scaleDownUpdateMode = hvpav1alpha1.UpdateModeOff
	}

	if k.values.SNI.PodMutatorEnabled {
		vpaContainerResourcePolicies = append(vpaContainerResourcePolicies, vpaautoscalingv1.ContainerResourcePolicy{
			ContainerName:    containerNameAPIServerProxyPodMutator,
			Mode:             &containerPolicyOff,
			ControlledValues: &controlledValues,
		})
	}

	if k.values.Autoscaling.MaxReplicas > k.values.Autoscaling.MinReplicas {
		weightBasedScalingIntervals = append(weightBasedScalingIntervals, hvpav1alpha1.WeightBasedScalingInterval{
			VpaWeight:         hvpav1alpha1.HpaOnly,
			StartReplicaCount: k.values.Autoscaling.MinReplicas,
			LastReplicaCount:  k.values.Autoscaling.MaxReplicas - 1,
		})
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client.Client(), hvpa, func() error {
		hvpa.Spec.Replicas = pointer.Int32Ptr(1)
		hvpa.Spec.Hpa = hvpav1alpha1.HpaSpec{
			Selector: &metav1.LabelSelector{MatchLabels: hpaLabels},
			Deploy:   true,
			ScaleUp: hvpav1alpha1.ScaleType{
				UpdatePolicy: hvpav1alpha1.UpdatePolicy{
					UpdateMode: &updateModeAuto,
				},
			},
			ScaleDown: hvpav1alpha1.ScaleType{
				UpdatePolicy: hvpav1alpha1.UpdatePolicy{
					UpdateMode: &updateModeAuto, // HPA does not work with update mode 'Off' and needs to be always 'Auto' even though scale down is disabled.
				},
			},
			Template: hvpav1alpha1.HpaTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Labels: hpaLabels,
				},
				Spec: hvpav1alpha1.HpaTemplateSpec{
					MinReplicas: &k.values.Autoscaling.MinReplicas,
					MaxReplicas: k.values.Autoscaling.MaxReplicas,
					Metrics:     hpaMetrics,
				},
			},
		}
		hvpa.Spec.Vpa = hvpav1alpha1.VpaSpec{
			Selector: &metav1.LabelSelector{MatchLabels: vpaLabels},
			Deploy:   true,
			ScaleUp: hvpav1alpha1.ScaleType{
				UpdatePolicy: hvpav1alpha1.UpdatePolicy{
					UpdateMode: &updateModeAuto,
				},
				StabilizationDuration: pointer.StringPtr("3m"),
				MinChange: hvpav1alpha1.ScaleParams{
					CPU: hvpav1alpha1.ChangeParams{
						Value:      pointer.StringPtr("300m"),
						Percentage: pointer.Int32Ptr(80),
					},
					Memory: hvpav1alpha1.ChangeParams{
						Value:      pointer.StringPtr("200M"),
						Percentage: pointer.Int32Ptr(80),
					},
				},
			},
			ScaleDown: hvpav1alpha1.ScaleType{
				UpdatePolicy: hvpav1alpha1.UpdatePolicy{
					UpdateMode: &scaleDownUpdateMode,
				},
				StabilizationDuration: pointer.StringPtr("15m"),
				MinChange: hvpav1alpha1.ScaleParams{
					CPU: hvpav1alpha1.ChangeParams{
						Value:      pointer.StringPtr("300m"),
						Percentage: pointer.Int32Ptr(80),
					},
					Memory: hvpav1alpha1.ChangeParams{
						Value:      pointer.StringPtr("200M"),
						Percentage: pointer.Int32Ptr(80),
					},
				},
			},
			LimitsRequestsGapScaleParams: hvpav1alpha1.ScaleParams{
				CPU: hvpav1alpha1.ChangeParams{
					Value:      pointer.StringPtr("1"),
					Percentage: pointer.Int32Ptr(70),
				},
				Memory: hvpav1alpha1.ChangeParams{
					Value:      pointer.StringPtr("1G"),
					Percentage: pointer.Int32Ptr(70),
				},
			},
			Template: hvpav1alpha1.VpaTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Labels: vpaLabels,
				},
				Spec: hvpav1alpha1.VpaTemplateSpec{
					ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
						ContainerPolicies: vpaContainerResourcePolicies,
					},
				},
			},
		}
		hvpa.Spec.WeightBasedScalingIntervals = weightBasedScalingIntervals
		hvpa.Spec.TargetRef = &autoscalingv2beta1.CrossVersionObjectReference{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
			Name:       deployment.Name,
		}
		return nil
	})
	return err
}
