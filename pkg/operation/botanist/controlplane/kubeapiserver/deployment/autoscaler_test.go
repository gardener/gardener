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

package deployment_test

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/version"

	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	autoscalingv2beta2 "k8s.io/api/autoscaling/v2beta2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
)

var hvpaUpdateMode = string(autoscalingv1beta2.UpdateModeAuto)

func expectAutoscaler(ctx context.Context, valuesProvider KubeAPIServerValuesProvider, deploymentReplicas *int32) {
	var (
		minReplicas          int32 = 1
		maxReplicas          int32 = 4
		managedSeedAPIServer       = valuesProvider.GetManagedSeedAPIServer()
	)

	if managedSeedAPIServer != nil && managedSeedAPIServer.Autoscaler != nil {
		minReplicas = *managedSeedAPIServer.Autoscaler.MinReplicas
		maxReplicas = managedSeedAPIServer.Autoscaler.MaxReplicas
	}

	if valuesProvider.IsHvpaEnabled() {
		expectHVPA(ctx, valuesProvider, deploymentReplicas, minReplicas, maxReplicas)
		return
	}
	expectBothVPAHPA(ctx, valuesProvider, deploymentReplicas, minReplicas, maxReplicas)
}

func expectHVPA(ctx context.Context, valuesProvider KubeAPIServerValuesProvider, deploymentReplicas *int32, minReplicas, maxReplicas int32) {
	mockSeedInterface.EXPECT().Version().Return(valuesProvider.GetSeedKubernetesVersion())
	mockSeedClient.EXPECT().Delete(ctx, &autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-vpa", Namespace: defaultSeedNamespace}})

	seedVersionGE112, err := version.CompareVersions(valuesProvider.GetSeedKubernetesVersion(), ">=", "1.12")
	Expect(err).ToNot(HaveOccurred())

	// autoscaling/v2beta1 is deprecated in favor of autoscaling/v2beta2 beginning with v1.19
	// ref https://github.com/kubernetes/kubernetes/pull/90463
	hpaObjectMeta := kutil.ObjectMeta(defaultSeedNamespace, "kube-apiserver")
	if seedVersionGE112 {
		mockSeedClient.EXPECT().Delete(ctx, &autoscalingv2beta2.HorizontalPodAutoscaler{ObjectMeta: hpaObjectMeta})
	} else {
		mockSeedClient.EXPECT().Delete(ctx, &autoscalingv2beta1.HorizontalPodAutoscaler{ObjectMeta: hpaObjectMeta})
	}

	if deploymentReplicas == nil || *deploymentReplicas == 0 {
		return
	}

	mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "kube-apiserver"), gomock.AssignableToTypeOf(&hvpav1alpha1.Hvpa{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "foo"))

	expectedHVPA := &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver", Namespace: defaultSeedNamespace}}
	expectedHVPA.Spec.Replicas = pointer.Int32Ptr(1)

	if valuesProvider.GetMaintenanceWindow() != nil {
		expectedHVPA.Spec.MaintenanceTimeWindow = &hvpav1alpha1.MaintenanceTimeWindow{
			Begin: valuesProvider.GetMaintenanceWindow().Begin,
			End:   valuesProvider.GetMaintenanceWindow().End,
		}
	}

	expectedHVPA.Spec.Hpa = hvpav1alpha1.HpaSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"role": "apiserver-hpa",
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
					"role": "apiserver-hpa",
				},
			},
			Spec: hvpav1alpha1.HpaTemplateSpec{
				MinReplicas: &minReplicas,
				MaxReplicas: maxReplicas,
				Metrics: []autoscalingv2beta1.MetricSpec{
					{
						Type: "Resource",
						Resource: &autoscalingv2beta1.ResourceMetricSource{
							Name:                     "cpu",
							TargetAverageUtilization: pointer.Int32Ptr(80),
						},
					},
				},
			},
		},
	}

	// Set for shooted seeds
	if valuesProvider.GetManagedSeedAPIServer() != nil {
		expectedHVPA.Spec.Hpa.Template.Spec.Metrics = append(expectedHVPA.Spec.Hpa.Template.Spec.Metrics,
			autoscalingv2beta1.MetricSpec{
				Type: "Resource",
				Resource: &autoscalingv2beta1.ResourceMetricSource{
					Name:                     "memory",
					TargetAverageUtilization: pointer.Int32Ptr(80),
				},
			},
		)
	}

	var (
		containerScalingModeOff = autoscalingv1beta2.ContainerScalingModeOff
		updateModeAuto          = hvpav1alpha1.UpdateModeAuto
	)

	expectedHVPA.Spec.Vpa = hvpav1alpha1.VpaSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"role": "apiserver-vpa",
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
					"role": "apiserver-vpa",
				},
			},
			Spec: hvpav1alpha1.VpaTemplateSpec{
				ResourcePolicy: &autoscalingv1beta2.PodResourcePolicy{
					ContainerPolicies: []autoscalingv1beta2.ContainerResourcePolicy{
						{
							ContainerName: "kube-apiserver",
							MaxAllowed: corev1.ResourceList{
								"memory": resource.MustParse("25G"),
								"cpu":    resource.MustParse("8"),
							},
							MinAllowed: corev1.ResourceList{
								"memory": resource.MustParse("400M"),
								"cpu":    resource.MustParse("300m"),
							},
						},
						{
							ContainerName: "vpn-seed",
							Mode:          &containerScalingModeOff,
						},
					},
				},
			},
		},
	}

	if valuesProvider.GetSNIValues().SNIPodMutatorEnabled {
		expectedHVPA.Spec.Vpa.Template.Spec.ResourcePolicy.ContainerPolicies = append(expectedHVPA.Spec.Vpa.Template.Spec.ResourcePolicy.ContainerPolicies,
			autoscalingv1beta2.ContainerResourcePolicy{
				ContainerName: "apiserver-proxy-pod-mutator",
				Mode:          &containerScalingModeOff,
			})
	}

	expectedHVPA.Spec.WeightBasedScalingIntervals = []hvpav1alpha1.WeightBasedScalingInterval{}
	if maxReplicas > minReplicas {
		expectedHVPA.Spec.WeightBasedScalingIntervals = append(expectedHVPA.Spec.WeightBasedScalingIntervals, hvpav1alpha1.WeightBasedScalingInterval{
			VpaWeight:         hvpav1alpha1.VpaWeight(0),
			StartReplicaCount: minReplicas,
			LastReplicaCount:  maxReplicas - 1,
		})
	}

	expectedHVPA.Spec.WeightBasedScalingIntervals = append(expectedHVPA.Spec.WeightBasedScalingIntervals, hvpav1alpha1.WeightBasedScalingInterval{
		VpaWeight:         100,
		StartReplicaCount: maxReplicas,
		LastReplicaCount:  maxReplicas,
	})

	expectedHVPA.Spec.TargetRef = &autoscalingv2beta1.CrossVersionObjectReference{
		APIVersion: appsv1.SchemeGroupVersion.String(),
		Kind:       "Deployment",
		Name:       "kube-apiserver",
	}

	mockSeedClient.EXPECT().Create(ctx, expectedHVPA).Times(1)
}

func expectBothVPAHPA(ctx context.Context, valuesProvider KubeAPIServerValuesProvider, deploymentReplicas *int32, minReplicas, maxReplicas int32) {
	mockSeedClient.EXPECT().Delete(ctx, &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver", Namespace: defaultSeedNamespace}}).Return(nil)

	hpaLabels := map[string]string{
		"role": "apiserver-hpa",
	}

	hpaToDelete := autoscalingv2beta1.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-apiserver-hpa-xzy",
			Namespace: defaultSeedNamespace,
		},
	}

	mockSeedClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&autoscalingv2beta1.HorizontalPodAutoscalerList{}), client.InNamespace(defaultSeedNamespace), client.MatchingLabels(hpaLabels), client.Limit(1)).DoAndReturn(func(_ context.Context, list *autoscalingv2beta1.HorizontalPodAutoscalerList, _ ...client.ListOption) error {
		*list = autoscalingv2beta1.HorizontalPodAutoscalerList{Items: []autoscalingv2beta1.HorizontalPodAutoscaler{
			hpaToDelete,
		}}
		return nil
	})
	mockSeedClient.EXPECT().Delete(ctx, &hpaToDelete).Return(nil)

	vpaLabels := map[string]string{
		"role": "apiserver-vpa",
	}

	vpaToDelete := autoscalingv1beta2.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-apiserver-vpa-xzy",
			Namespace: defaultSeedNamespace,
		},
	}
	mockSeedClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscalerList{}), client.InNamespace(defaultSeedNamespace), client.MatchingLabels(vpaLabels), client.Limit(1)).DoAndReturn(func(_ context.Context, list *autoscalingv1beta2.VerticalPodAutoscalerList, _ ...client.ListOption) error {
		*list = autoscalingv1beta2.VerticalPodAutoscalerList{Items: []autoscalingv1beta2.VerticalPodAutoscaler{
			vpaToDelete,
		}}
		return nil
	})
	mockSeedClient.EXPECT().Delete(ctx, &vpaToDelete).Return(nil)

	mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "kube-apiserver-vpa"), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "foo"))

	vpaUpdateMode := autoscalingv1beta2.UpdateModeOff
	expectedVPA := &autoscalingv1beta2.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-vpa",
			Namespace: defaultSeedNamespace,
		},
		Spec: autoscalingv1beta2.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
				Name:       "kube-apiserver",
			},
			UpdatePolicy: &autoscalingv1beta2.PodUpdatePolicy{
				UpdateMode: &vpaUpdateMode,
			},
		},
	}

	mockSeedClient.EXPECT().Create(ctx, expectedVPA).Times(1)

	if deploymentReplicas == nil || *deploymentReplicas == 0 {
		return
	}

	mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "kube-apiserver"), gomock.AssignableToTypeOf(&autoscalingv2beta1.HorizontalPodAutoscaler{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "foo"))

	expectedHPA := &autoscalingv2beta1.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver",
			Namespace: defaultSeedNamespace,
		},
		Spec: autoscalingv2beta1.HorizontalPodAutoscalerSpec{
			MinReplicas: &minReplicas,
			MaxReplicas: maxReplicas,
			ScaleTargetRef: autoscalingv2beta1.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
				Name:       "kube-apiserver",
			},
			Metrics: []autoscalingv2beta1.MetricSpec{
				{
					Type: "Resource",
					Resource: &autoscalingv2beta1.ResourceMetricSource{
						Name:                     "cpu",
						TargetAverageUtilization: pointer.Int32Ptr(80),
					},
				},
				{
					Type: "Resource",
					Resource: &autoscalingv2beta1.ResourceMetricSource{
						Name:                     "memory",
						TargetAverageUtilization: pointer.Int32Ptr(80),
					},
				},
			},
		},
	}
	mockSeedClient.EXPECT().Create(ctx, expectedHPA).Times(1)
}
