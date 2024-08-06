// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"context"
	"fmt"

	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component/apiserver"
	"github.com/gardener/gardener/pkg/controllerutils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

func (k *kubeAPIServer) emptyHVPA() *hvpav1alpha1.Hvpa {
	return &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: k.values.NamePrefix + v1beta1constants.DeploymentNameKubeAPIServer, Namespace: k.namespace}}
}

func (k *kubeAPIServer) reconcileHVPA(ctx context.Context, hvpa *hvpav1alpha1.Hvpa, deployment *appsv1.Deployment) error {
	if k.values.Autoscaling.Mode != apiserver.AutoscalingModeHVPA ||
		k.values.Autoscaling.Replicas == nil ||
		*k.values.Autoscaling.Replicas == 0 {
		return kubernetesutils.DeleteObject(ctx, k.client.Client(), hvpa)
	}

	var (
		hpaLabels           = map[string]string{v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer + "-hpa"}
		vpaLabels           = map[string]string{v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer + "-vpa"}
		updateModeAuto      = hvpav1alpha1.UpdateModeAuto
		scaleDownUpdateMode = updateModeAuto
		hpaMetrics          = []autoscalingv2beta1.MetricSpec{
			{
				Type: autoscalingv2beta1.ResourceMetricSourceType,
				Resource: &autoscalingv2beta1.ResourceMetricSource{
					Name:                     corev1.ResourceCPU,
					TargetAverageUtilization: ptr.To(hpaTargetAverageUtilizationCPU),
				},
			},
		}
		kubeAPIServerMinAllowed = corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("200M"),
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
				TargetAverageUtilization: ptr.To(hpaTargetAverageUtilizationMemory),
			},
		})
	}

	if k.values.Autoscaling.ScaleDownDisabled {
		scaleDownUpdateMode = hvpav1alpha1.UpdateModeOff
	}

	if k.values.Autoscaling.MaxReplicas > k.values.Autoscaling.MinReplicas {
		weightBasedScalingIntervals = append(weightBasedScalingIntervals, hvpav1alpha1.WeightBasedScalingInterval{
			VpaWeight:         hvpav1alpha1.HpaOnly,
			StartReplicaCount: k.values.Autoscaling.MinReplicas,
			LastReplicaCount:  k.values.Autoscaling.MaxReplicas - 1,
		})
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client.Client(), hvpa, func() error {
		metav1.SetMetaDataLabel(&hvpa.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigType, resourcesv1alpha1.HighAvailabilityConfigTypeServer)
		hvpa.Spec.Replicas = ptr.To[int32](1)
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
					UpdateMode: &scaleDownUpdateMode,
				},
				StabilizationDuration: ptr.To("15m"),
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
						ContainerPolicies: k.computeVerticalPodAutoscalerContainerResourcePolicies(kubeAPIServerMinAllowed),
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

func (k *kubeAPIServer) computeVerticalPodAutoscalerContainerResourcePolicies(kubeAPIServerMinAllowed corev1.ResourceList) []vpaautoscalingv1.ContainerResourcePolicy {
	var (
		vpaContainerResourcePolicies = []vpaautoscalingv1.ContainerResourcePolicy{
			{
				ContainerName:    ContainerNameKubeAPIServer,
				MinAllowed:       kubeAPIServerMinAllowed,
				ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
			},
		}
	)

	if k.values.VPN.HighAvailabilityEnabled {
		for i := 0; i < k.values.VPN.HighAvailabilityNumberOfSeedServers; i++ {
			vpaContainerResourcePolicies = append(vpaContainerResourcePolicies, vpaautoscalingv1.ContainerResourcePolicy{
				ContainerName: fmt.Sprintf("%s-%d", containerNameVPNSeedClient, i),
				Mode:          ptr.To(vpaautoscalingv1.ContainerScalingModeOff),
			})
		}
		vpaContainerResourcePolicies = append(vpaContainerResourcePolicies, vpaautoscalingv1.ContainerResourcePolicy{
			ContainerName: containerNameVPNPathController,
			Mode:          ptr.To(vpaautoscalingv1.ContainerScalingModeOff),
		})
	}

	return vpaContainerResourcePolicies
}
