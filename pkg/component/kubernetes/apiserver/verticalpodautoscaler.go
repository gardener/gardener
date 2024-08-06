// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/apiserver"
	"github.com/gardener/gardener/pkg/controllerutils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

func (k *kubeAPIServer) emptyVerticalPodAutoscaler() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: k.values.NamePrefix + v1beta1constants.DeploymentNameKubeAPIServer + "-vpa", Namespace: k.namespace}}
}

func (k *kubeAPIServer) reconcileVerticalPodAutoscaler(ctx context.Context, verticalPodAutoscaler *vpaautoscalingv1.VerticalPodAutoscaler, deployment *appsv1.Deployment) error {
	switch k.values.Autoscaling.Mode {
	case apiserver.AutoscalingModeHVPA:
		return kubernetesutils.DeleteObject(ctx, k.client.Client(), verticalPodAutoscaler)
	case apiserver.AutoscalingModeVPAAndHPA:
		return k.reconcileVerticalPodAutoscalerInVPAAndHPAMode(ctx, verticalPodAutoscaler, deployment)
	default:
		return k.reconcileVerticalPodAutoscalerInBaselineMode(ctx, verticalPodAutoscaler, deployment)
	}
}

func (k *kubeAPIServer) reconcileVerticalPodAutoscalerInBaselineMode(ctx context.Context, verticalPodAutoscaler *vpaautoscalingv1.VerticalPodAutoscaler, deployment *appsv1.Deployment) error {
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client.Client(), verticalPodAutoscaler, func() error {
		verticalPodAutoscaler.Spec = vpaautoscalingv1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
				Name:       deployment.Name,
			},
			UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeOff),
			},
			ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
				ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
					ContainerName:    vpaautoscalingv1.DefaultContainerResourcePolicy,
					ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
				}},
			},
		}
		return nil
	})
	return err
}

func (k *kubeAPIServer) reconcileVerticalPodAutoscalerInVPAAndHPAMode(ctx context.Context, verticalPodAutoscaler *vpaautoscalingv1.VerticalPodAutoscaler, deployment *appsv1.Deployment) error {
	kubeAPIServerMinAllowed := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("20m"),
		corev1.ResourceMemory: resource.MustParse("200M"),
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client.Client(), verticalPodAutoscaler, func() error {
		verticalPodAutoscaler.Spec = vpaautoscalingv1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
				Name:       deployment.Name,
			},
			UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
			},
			ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
				ContainerPolicies: k.computeVerticalPodAutoscalerContainerResourcePolicies(kubeAPIServerMinAllowed),
			},
		}

		if k.values.Autoscaling.ScaleDownDisabled {
			metav1.SetMetaDataLabel(&verticalPodAutoscaler.ObjectMeta, v1beta1constants.LabelVPAEvictionRequirementsController, v1beta1constants.EvictionRequirementManagedByController)
			metav1.SetMetaDataAnnotation(&verticalPodAutoscaler.ObjectMeta, v1beta1constants.AnnotationVPAEvictionRequirementDownscaleRestriction, v1beta1constants.EvictionRequirementNever)
		} else {
			delete(verticalPodAutoscaler.GetLabels(), v1beta1constants.LabelVPAEvictionRequirementsController)
			delete(verticalPodAutoscaler.GetAnnotations(), v1beta1constants.AnnotationVPAEvictionRequirementDownscaleRestriction)
		}

		return nil
	})

	return err
}
