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
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Port is the port exposed by the kube-apiserver.
	Port = 443

	containerNameKubeAPIServer            = "kube-apiserver"
	containerNameVPNSeed                  = "vpn-seed"
	containerNameAPIServerProxyPodMutator = "apiserver-proxy-pod-mutator"
)

// Interface contains functions for a kube-apiserver deployer.
type Interface interface {
	component.DeployWaiter
	// GetValues returns the current configuration values of the deployer.
	GetValues() Values
	// SetAutoscalingReplicas sets the Replicas field in the AutoscalingConfig of the Values of the deployer.
	SetAutoscalingReplicas(*int32)
}

// Values contains configuration values for the kube-apiserver resources.
type Values struct {
	// Autoscaling contains information for configuring autoscaling settings for the kube-apiserver.
	Autoscaling AutoscalingConfig
	// SNI contains information for configuring SNI settings for the kube-apiserver.
	SNI SNIConfig
}

// AutoscalingConfig contains information for configuring autoscaling settings for the kube-apiserver.
type AutoscalingConfig struct {
	// HVPAEnabled states whether an HVPA object shall be deployed. If false, HPA and VPA will be used.
	HVPAEnabled bool
	// Replicas is the number of pod replicas for the kube-apiserver.
	Replicas *int32
	// MinReplicas are the minimum Replicas for horizontal autoscaling.
	MinReplicas int32
	// MaxReplicas are the maximum Replicas for horizontal autoscaling.
	MaxReplicas int32
	// UseMemoryMetricForHvpaHPA states whether the memory metric shall be used when the HPA is configured in an HVPA
	// resource.
	UseMemoryMetricForHvpaHPA bool
	// ScaleDownDisabledForHvpa states whether scale-down shall be disabled when HPA or VPA are configured in an HVPA
	// resource.
	ScaleDownDisabledForHvpa bool
}

// SNIConfig contains information for configuring SNI settings for the kube-apiserver.
type SNIConfig struct {
	// PodMutatorEnabled states whether the pod mutator is enabled.
	PodMutatorEnabled bool
}

// New creates a new instance of DeployWaiter for the kube-apiserver.
func New(client client.Client, namespace string, values Values) Interface {
	return &kubeAPIServer{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type kubeAPIServer struct {
	client    client.Client
	namespace string
	values    Values
}

func (k *kubeAPIServer) Deploy(ctx context.Context) error {
	var (
		needsHPA  = !k.values.Autoscaling.HVPAEnabled && k.values.Autoscaling.Replicas != nil && *k.values.Autoscaling.Replicas > 0
		needsVPA  = !k.values.Autoscaling.HVPAEnabled
		needsHVPA = k.values.Autoscaling.HVPAEnabled && k.values.Autoscaling.Replicas != nil && *k.values.Autoscaling.Replicas > 0

		hpaTargetAverageUtilizationCPU    int32 = 80
		hpaTargetAverageUtilizationMemory int32 = 80
		vpaUpdateMode                           = autoscalingv1beta2.UpdateModeOff
		pdbMaxUnavailable                       = intstr.FromInt(1)

		horizontalPodAutoscaler = k.emptyHorizontalPodAutoscaler()
		verticalPodAutoscaler   = k.emptyVerticalPodAutoscaler()
		hvpa                    = k.emptyHVPA()
		podDisruptionBudget     = k.emptyPodDisruptionBudget()
	)

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client, podDisruptionBudget, func() error {
		podDisruptionBudget.Labels = getLabels()
		podDisruptionBudget.Spec = policyv1beta1.PodDisruptionBudgetSpec{
			MaxUnavailable: &pdbMaxUnavailable,
			Selector:       &metav1.LabelSelector{MatchLabels: getLabels()},
		}
		return nil
	}); err != nil {
		return err
	}

	if needsHPA {
		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client, horizontalPodAutoscaler, func() error {
			horizontalPodAutoscaler.Spec = autoscalingv2beta1.HorizontalPodAutoscalerSpec{
				MinReplicas: &k.values.Autoscaling.MinReplicas,
				MaxReplicas: k.values.Autoscaling.MaxReplicas,
				ScaleTargetRef: autoscalingv2beta1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					// TODO: Replace with `deployment.Name` once the Deployment is generated by this component.
					Name: v1beta1constants.DeploymentNameKubeAPIServer,
				},
				Metrics: []autoscalingv2beta1.MetricSpec{
					{
						Type: autoscalingv2beta1.ResourceMetricSourceType,
						Resource: &autoscalingv2beta1.ResourceMetricSource{
							Name:                     corev1.ResourceCPU,
							TargetAverageUtilization: &hpaTargetAverageUtilizationCPU,
						},
					},
					{
						Type: autoscalingv2beta1.ResourceMetricSourceType,
						Resource: &autoscalingv2beta1.ResourceMetricSource{
							Name:                     corev1.ResourceMemory,
							TargetAverageUtilization: &hpaTargetAverageUtilizationMemory,
						},
					},
				},
			}
			return nil
		}); err != nil {
			return err
		}
	} else {
		if err := kutil.DeleteObject(ctx, k.client, horizontalPodAutoscaler); err != nil {
			return err
		}
	}

	if needsVPA {
		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client, verticalPodAutoscaler, func() error {
			verticalPodAutoscaler.Spec = autoscalingv1beta2.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					// TODO: Replace with `deployment.Name` once the Deployment is generated by this component.
					Name: v1beta1constants.DeploymentNameKubeAPIServer,
				},
				UpdatePolicy: &autoscalingv1beta2.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
			}
			return nil
		}); err != nil {
			return err
		}
	} else {
		if err := kutil.DeleteObject(ctx, k.client, verticalPodAutoscaler); err != nil {
			return err
		}
	}

	if needsHVPA {
		var (
			hpaLabels           = map[string]string{v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer + "-hpa"}
			vpaLabels           = map[string]string{v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer + "-vpa"}
			updateModeAuto      = hvpav1alpha1.UpdateModeAuto
			scaleDownUpdateMode = updateModeAuto
			containerPolicyOff  = autoscalingv1beta2.ContainerScalingModeOff
			hpaMetrics          = []autoscalingv2beta1.MetricSpec{
				{
					Type: autoscalingv2beta1.ResourceMetricSourceType,
					Resource: &autoscalingv2beta1.ResourceMetricSource{
						Name:                     corev1.ResourceCPU,
						TargetAverageUtilization: &hpaTargetAverageUtilizationCPU,
					},
				},
			}
			vpaContainerResourcePolicies = []autoscalingv1beta2.ContainerResourcePolicy{
				{
					ContainerName: containerNameKubeAPIServer,
					MinAllowed: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("300m"),
						corev1.ResourceMemory: resource.MustParse("400M"),
					},
					MaxAllowed: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("8"),
						corev1.ResourceMemory: resource.MustParse("25G"),
					},
				},
				{
					ContainerName: containerNameVPNSeed,
					Mode:          &containerPolicyOff,
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
					TargetAverageUtilization: &hpaTargetAverageUtilizationMemory,
				},
			})
		}

		if k.values.Autoscaling.ScaleDownDisabledForHvpa {
			scaleDownUpdateMode = hvpav1alpha1.UpdateModeOff
		}

		if k.values.SNI.PodMutatorEnabled {
			vpaContainerResourcePolicies = append(vpaContainerResourcePolicies, autoscalingv1beta2.ContainerResourcePolicy{
				ContainerName: containerNameAPIServerProxyPodMutator,
				Mode:          &containerPolicyOff,
			})
		}

		if k.values.Autoscaling.MaxReplicas > k.values.Autoscaling.MinReplicas {
			weightBasedScalingIntervals = append(weightBasedScalingIntervals, hvpav1alpha1.WeightBasedScalingInterval{
				VpaWeight:         hvpav1alpha1.HpaOnly,
				StartReplicaCount: k.values.Autoscaling.MinReplicas,
				LastReplicaCount:  k.values.Autoscaling.MaxReplicas - 1,
			})
		}

		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client, hvpa, func() error {
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
						UpdateMode: &scaleDownUpdateMode,
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
						ResourcePolicy: &autoscalingv1beta2.PodResourcePolicy{
							ContainerPolicies: vpaContainerResourcePolicies,
						},
					},
				},
			}
			hvpa.Spec.WeightBasedScalingIntervals = weightBasedScalingIntervals
			hvpa.Spec.TargetRef = &autoscalingv2beta1.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
				// TODO: Replace with `deployment.Name` once the Deployment is generated by this component.
				Name: v1beta1constants.DeploymentNameKubeAPIServer,
			}
			return nil
		}); err != nil {
			return err
		}
	} else {
		if err := kutil.DeleteObject(ctx, k.client, hvpa); err != nil {
			return err
		}
	}

	return nil
}

func (k *kubeAPIServer) Destroy(ctx context.Context) error {
	return kutil.DeleteObjects(ctx, k.client,
		k.emptyHorizontalPodAutoscaler(),
		k.emptyVerticalPodAutoscaler(),
		k.emptyHVPA(),
		k.emptyPodDisruptionBudget(),
	)
}

func (k *kubeAPIServer) Wait(_ context.Context) error        { return nil }
func (k *kubeAPIServer) WaitCleanup(_ context.Context) error { return nil }

func (k *kubeAPIServer) GetValues() Values {
	return k.values
}
func (k *kubeAPIServer) SetAutoscalingReplicas(replicas *int32) {
	k.values.Autoscaling.Replicas = replicas
}

func (k *kubeAPIServer) emptyHorizontalPodAutoscaler() *autoscalingv2beta1.HorizontalPodAutoscaler {
	return &autoscalingv2beta1.HorizontalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: k.namespace}}
}

func (k *kubeAPIServer) emptyVerticalPodAutoscaler() *autoscalingv1beta2.VerticalPodAutoscaler {
	return &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer + "-vpa", Namespace: k.namespace}}
}

func (k *kubeAPIServer) emptyHVPA() *hvpav1alpha1.Hvpa {
	return &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: k.namespace}}
}

func (k *kubeAPIServer) emptyPodDisruptionBudget() *policyv1beta1.PodDisruptionBudget {
	return &policyv1beta1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: k.namespace}}
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
		v1beta1constants.LabelRole: v1beta1constants.LabelAPIServer,
	}
}
