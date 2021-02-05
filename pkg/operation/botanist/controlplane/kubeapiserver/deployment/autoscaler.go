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

package deployment

import (
	"context"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/version"

	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	autoscalingv2beta2 "k8s.io/api/autoscaling/v2beta2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// nameKubeAPIServerHVPA is a constant for the name of the VHPA resource of the API server
	nameKubeAPIServerHVPA = "kube-apiserver"
	// nameKubeAPIServerVPA is a constant for the name of the HPA resource of the API server
	nameKubeAPIServerHPA = "kube-apiserver"
	// nameKubeAPIServerVPA is a constant for the name of the VPA resource of the API server
	nameKubeAPIServerVPA = "kube-apiserver-vpa"
	// labelRoleHPA is a constant for the value of a label with key 'role' whose value is 'apiserver-hpa'.
	labelRoleHPA = "apiserver-hpa"
	// labelRoleVPA is a constant for the value of a label with key 'role' whose value is 'apiserver-vpa'.
	labelRoleVPA = "apiserver-vpa"
	// referenceKindDeployment is a constant for the Kind attribute of autoscalingv2beta1.CrossVersionObjectReference with value "Deployment"
	// see: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	referenceKindDeployment = "Deployment"
)

var (
	vpaUpdateMode  = autoscalingv1beta2.UpdateModeOff
	hvpaUpdateMode = string(autoscalingv1beta2.UpdateModeAuto)
)

func (k *kubeAPIServer) deployAutoscaler(ctx context.Context) error {
	var (
		minReplicas int32 = 1
		maxReplicas int32 = 4
	)

	if k.managedSeed != nil && k.managedSeed.Autoscaler != nil {
		minReplicas = *k.managedSeed.Autoscaler.MinReplicas
		maxReplicas = k.managedSeed.Autoscaler.MaxReplicas
	}

	if k.hvpaEnabled {
		return k.deployHVPA(ctx, minReplicas, maxReplicas)
	}

	return k.deployBothVPAHPA(ctx, minReplicas, maxReplicas)
}

func (k *kubeAPIServer) deployHVPA(ctx context.Context, minReplicas, maxReplicas int32) error {
	// If HVPA feature gate is enabled then we should delete the old HPA and VPA resources as
	// the HVPA controller will create its own for the kube-apiserver deployment.
	objects := []client.Object{
		k.emptyVPA(),
	}

	// autoscaling/v2beta1 is deprecated in favor of autoscaling/v2beta2 beginning with v1.19
	// ref https://github.com/kubernetes/kubernetes/pull/90463
	hpaObjectMeta := kutil.ObjectMeta(k.seedNamespace, nameKubeAPIServerHPA)
	seedVersionGE112, err := version.CompareVersions(k.seedClient.Version(), ">=", "1.12")
	if err != nil {
		return err
	}

	if seedVersionGE112 {
		objects = append(objects, &autoscalingv2beta2.HorizontalPodAutoscaler{ObjectMeta: hpaObjectMeta})
	} else {
		objects = append(objects, &autoscalingv2beta1.HorizontalPodAutoscaler{ObjectMeta: hpaObjectMeta})
	}

	if err := kutil.DeleteObjects(ctx, k.seedClient.Client(), objects...); err != nil {
		return err
	}

	if k.deploymentReplicas == nil || *k.deploymentReplicas == 0 {
		return nil
	}

	var hvpa = k.emptyHVPA()
	if _, err := controllerutil.CreateOrUpdate(ctx, k.seedClient.Client(), hvpa, func() error {
		hvpa.Spec.Replicas = pointer.Int32Ptr(1)

		if k.maintenanceWindow != nil {
			hvpa.Spec.MaintenanceTimeWindow = &hvpav1alpha1.MaintenanceTimeWindow{
				Begin: k.maintenanceWindow.Begin,
				End:   k.maintenanceWindow.End,
			}
		}

		hvpa.Spec.Hpa = hvpav1alpha1.HpaSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					v1beta1constants.LabelRole: labelRoleHPA,
				},
			},
			Deploy: true,
			ScaleUp: hvpav1alpha1.ScaleType{
				UpdatePolicy: hvpav1alpha1.UpdatePolicy{
					UpdateMode: &hvpaUpdateMode,
				},
			},
			ScaleDown: hvpav1alpha1.ScaleType{
				UpdatePolicy: hvpav1alpha1.UpdatePolicy{
					UpdateMode: &hvpaUpdateMode,
				},
			},
			Template: hvpav1alpha1.HpaTemplate{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						v1beta1constants.LabelRole: labelRoleHPA,
					},
				},
				Spec: hvpav1alpha1.HpaTemplateSpec{
					MinReplicas: &minReplicas,
					MaxReplicas: maxReplicas,
					Metrics: []autoscalingv2beta1.MetricSpec{
						{
							Type: autoscalingv2beta1.ResourceMetricSourceType,
							Resource: &autoscalingv2beta1.ResourceMetricSource{
								Name:                     corev1.ResourceCPU,
								TargetAverageUtilization: pointer.Int32Ptr(80),
							},
						},
					},
				},
			},
		}

		// Collect memory metrics only for shooted seeds
		if k.managedSeed != nil {
			hvpa.Spec.Hpa.Template.Spec.Metrics = append(hvpa.Spec.Hpa.Template.Spec.Metrics,
				autoscalingv2beta1.MetricSpec{
					Type: autoscalingv2beta1.ResourceMetricSourceType,
					Resource: &autoscalingv2beta1.ResourceMetricSource{
						Name:                     corev1.ResourceMemory,
						TargetAverageUtilization: pointer.Int32Ptr(80),
					},
				},
			)
		}

		var (
			updateModeAuto          = hvpav1alpha1.UpdateModeAuto
			containerScalingModeOff = autoscalingv1beta2.ContainerScalingModeOff
		)

		hvpa.Spec.Vpa = hvpav1alpha1.VpaSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					v1beta1constants.LabelRole: labelRoleVPA,
				},
			},
			Deploy: true,
			ScaleUp: hvpav1alpha1.ScaleType{
				UpdatePolicy: hvpav1alpha1.UpdatePolicy{
					UpdateMode: &updateModeAuto,
				},
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
				StabilizationDuration: pointer.StringPtr("3m"),
			},
			ScaleDown: hvpav1alpha1.ScaleType{
				UpdatePolicy: hvpav1alpha1.UpdatePolicy{
					UpdateMode: &updateModeAuto,
				},
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
				StabilizationDuration: pointer.StringPtr("15m"),
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
					Labels: map[string]string{
						v1beta1constants.LabelRole: labelRoleVPA,
					},
				},
				Spec: hvpav1alpha1.VpaTemplateSpec{
					ResourcePolicy: &autoscalingv1beta2.PodResourcePolicy{
						ContainerPolicies: []autoscalingv1beta2.ContainerResourcePolicy{
							{
								ContainerName: containerNameKubeAPIServer,
								MaxAllowed: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("25G"),
									corev1.ResourceCPU:    resource.MustParse("8"),
								},
								MinAllowed: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("400M"),
									corev1.ResourceCPU:    resource.MustParse("300m"),
								},
							},
							{
								ContainerName: containerNameVPNSeed,
								Mode:          &containerScalingModeOff,
							},
						},
					},
				},
			},
		}

		if k.sniValues.SNIPodMutatorEnabled {
			hvpa.Spec.Vpa.Template.Spec.ResourcePolicy.ContainerPolicies = append(hvpa.Spec.Vpa.Template.Spec.ResourcePolicy.ContainerPolicies,
				autoscalingv1beta2.ContainerResourcePolicy{
					ContainerName: containerNameApiserverProxyPodMutator,
					Mode:          &containerScalingModeOff,
				})
		}

		hvpa.Spec.WeightBasedScalingIntervals = []hvpav1alpha1.WeightBasedScalingInterval{}
		if maxReplicas > minReplicas {
			hvpa.Spec.WeightBasedScalingIntervals = append(hvpa.Spec.WeightBasedScalingIntervals, hvpav1alpha1.WeightBasedScalingInterval{
				VpaWeight:         hvpav1alpha1.VpaWeight(0),
				StartReplicaCount: minReplicas,
				LastReplicaCount:  maxReplicas - 1,
			})
		}

		hvpa.Spec.WeightBasedScalingIntervals = append(hvpa.Spec.WeightBasedScalingIntervals, hvpav1alpha1.WeightBasedScalingInterval{
			VpaWeight:         hvpav1alpha1.VpaOnly,
			StartReplicaCount: maxReplicas,
			LastReplicaCount:  maxReplicas,
		})

		hvpa.Spec.TargetRef = &autoscalingv2beta1.CrossVersionObjectReference{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       referenceKindDeployment,
			Name:       v1beta1constants.DeploymentNameKubeAPIServer,
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (k *kubeAPIServer) deployBothVPAHPA(ctx context.Context, minReplicas, maxReplicas int32) error {
	// If HVPA is disabled, delete any HVPA that was already deployed
	if err := k.deleteHVPA(ctx); err != nil {
		return err
	}

	var (
		vpa = k.emptyVPA()
		hpa = k.emptyHPA()
	)

	if _, err := controllerutil.CreateOrUpdate(ctx, k.seedClient.Client(), vpa, func() error {
		vpa.Spec.TargetRef = &autoscalingv1.CrossVersionObjectReference{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       referenceKindDeployment,
			Name:       v1beta1constants.DeploymentNameKubeAPIServer,
		}
		vpa.Spec.UpdatePolicy = &autoscalingv1beta2.PodUpdatePolicy{
			UpdateMode: &vpaUpdateMode,
		}
		return nil
	}); err != nil {
		return err
	}

	if k.deploymentReplicas == nil || *k.deploymentReplicas == 0 {
		return nil
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, k.seedClient.Client(), hpa, func() error {
		hpa.Spec.MinReplicas = &minReplicas
		hpa.Spec.MaxReplicas = maxReplicas
		hpa.Spec.ScaleTargetRef = autoscalingv2beta1.CrossVersionObjectReference{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       referenceKindDeployment,
			Name:       v1beta1constants.DeploymentNameKubeAPIServer,
		}
		hpa.Spec.Metrics = []autoscalingv2beta1.MetricSpec{
			{
				Type: autoscalingv2beta1.ResourceMetricSourceType,
				Resource: &autoscalingv2beta1.ResourceMetricSource{
					Name:                     corev1.ResourceCPU,
					TargetAverageUtilization: pointer.Int32Ptr(80),
				},
			},
			{
				Type: autoscalingv2beta1.ResourceMetricSourceType,
				Resource: &autoscalingv2beta1.ResourceMetricSource{
					Name:                     corev1.ResourceMemory,
					TargetAverageUtilization: pointer.Int32Ptr(80),
				},
			},
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

// deleteHVPA deletes the HVPA resource and deployed VPA and HPA resources
func (k *kubeAPIServer) deleteHVPA(ctx context.Context) error {
	hvpa := k.emptyHVPA()
	if err := k.seedClient.Client().Delete(ctx, hvpa); err != nil {
		if !apierrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			return err
		}
	}

	// delete hpa and vpa resources deployed by hvpa
	var (
		hpaList   = &autoscalingv2beta1.HorizontalPodAutoscalerList{}
		vpaList   = &autoscalingv1beta2.VerticalPodAutoscalerList{}
		hpaLabels = map[string]string{
			v1beta1constants.LabelRole: labelRoleHPA,
		}
		vpaLabels = map[string]string{
			v1beta1constants.LabelRole: labelRoleVPA,
		}
	)

	if err := k.seedClient.Client().List(ctx, hpaList, client.InNamespace(k.seedNamespace), client.MatchingLabels(hpaLabels), client.Limit(1)); err != nil {
		return err
	}

	if len(hpaList.Items) > 0 {
		if err := k.seedClient.Client().Delete(ctx, &hpaList.Items[0]); err != nil {
			return err
		}
	}

	if err := k.seedClient.Client().List(ctx, vpaList, client.InNamespace(k.seedNamespace), client.MatchingLabels(vpaLabels), client.Limit(1)); err != nil {
		return err
	}

	if len(vpaList.Items) > 0 {
		if err := k.seedClient.Client().Delete(ctx, &vpaList.Items[0]); err != nil {
			return err
		}
	}

	return nil
}

func (k *kubeAPIServer) emptyHVPA() *hvpav1alpha1.Hvpa {
	return &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: nameKubeAPIServerHVPA, Namespace: k.seedNamespace}}
}

func (k *kubeAPIServer) emptyHPA() *autoscalingv2beta1.HorizontalPodAutoscaler {
	return &autoscalingv2beta1.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: nameKubeAPIServerHPA, Namespace: k.seedNamespace}}
}

func (k *kubeAPIServer) emptyVPA() *autoscalingv1beta2.VerticalPodAutoscaler {
	return &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: nameKubeAPIServerVPA, Namespace: k.seedNamespace}}
}
