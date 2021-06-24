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

package kubeapiserver_test

import (
	"context"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/onsi/ginkgo/extensions/table"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("KubeAPIServer", func() {
	var (
		ctx = context.TODO()

		namespace     = "some-namespace"
		vpaUpdateMode = autoscalingv1beta2.UpdateModeOff

		c    client.Client
		kapi Interface

		horizontalPodAutoscaler *autoscalingv2beta1.HorizontalPodAutoscaler
		verticalPodAutoscaler   *autoscalingv1beta2.VerticalPodAutoscaler
		podDisruptionBudget     *policyv1beta1.PodDisruptionBudget
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		kapi = New(c, namespace, Values{})

		horizontalPodAutoscaler = &autoscalingv2beta1.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver",
				Namespace: namespace,
			},
		}
		verticalPodAutoscaler = &autoscalingv1beta2.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver-vpa",
				Namespace: namespace,
			},
		}
		podDisruptionBudget = &policyv1beta1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver",
				Namespace: namespace,
			},
		}
	})

	Describe("#Deploy", func() {
		Describe("HorizontalPodAutoscaler", func() {
			DescribeTable("should delete the HPA resource",
				func(autoscalingConfig AutoscalingConfig) {
					kapi = New(c, namespace, Values{Autoscaling: autoscalingConfig})

					Expect(c.Create(ctx, horizontalPodAutoscaler)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(horizontalPodAutoscaler), horizontalPodAutoscaler)).To(Succeed())
					Expect(kapi.Deploy(ctx)).To(Succeed())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(horizontalPodAutoscaler), horizontalPodAutoscaler)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: autoscalingv2beta1.SchemeGroupVersion.Group, Resource: "horizontalpodautoscalers"}, horizontalPodAutoscaler.Name)))
				},

				Entry("HVPA is enabled", AutoscalingConfig{HVPAEnabled: true}),
				Entry("replicas is nil", AutoscalingConfig{HVPAEnabled: false, Replicas: nil}),
				Entry("replicas is 0", AutoscalingConfig{HVPAEnabled: false, Replicas: pointer.Int32(0)}),
			)

			It("should successfully deploy the HPA resource", func() {
				autoscalingConfig := AutoscalingConfig{
					HVPAEnabled: false,
					Replicas:    pointer.Int32(2),
					MinReplicas: 4,
					MaxReplicas: 6,
				}
				kapi = New(c, namespace, Values{Autoscaling: autoscalingConfig})

				Expect(c.Get(ctx, client.ObjectKeyFromObject(horizontalPodAutoscaler), horizontalPodAutoscaler)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: autoscalingv2beta1.SchemeGroupVersion.Group, Resource: "horizontalpodautoscalers"}, horizontalPodAutoscaler.Name)))
				Expect(kapi.Deploy(ctx)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(horizontalPodAutoscaler), horizontalPodAutoscaler)).To(Succeed())
				Expect(horizontalPodAutoscaler).To(DeepEqual(&autoscalingv2beta1.HorizontalPodAutoscaler{
					TypeMeta: metav1.TypeMeta{
						APIVersion: autoscalingv2beta1.SchemeGroupVersion.String(),
						Kind:       "HorizontalPodAutoscaler",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            horizontalPodAutoscaler.Name,
						Namespace:       horizontalPodAutoscaler.Namespace,
						ResourceVersion: "1",
					},
					Spec: autoscalingv2beta1.HorizontalPodAutoscalerSpec{
						MinReplicas: &autoscalingConfig.MinReplicas,
						MaxReplicas: autoscalingConfig.MaxReplicas,
						ScaleTargetRef: autoscalingv2beta1.CrossVersionObjectReference{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "kube-apiserver",
						},
						Metrics: []autoscalingv2beta1.MetricSpec{
							{
								Type: "Resource",
								Resource: &autoscalingv2beta1.ResourceMetricSource{
									Name:                     "cpu",
									TargetAverageUtilization: pointer.Int32(80),
								},
							},
							{
								Type: "Resource",
								Resource: &autoscalingv2beta1.ResourceMetricSource{
									Name:                     "memory",
									TargetAverageUtilization: pointer.Int32(80),
								},
							},
						},
					},
				}))
			})
		})

		Describe("VerticalPodAutoscaler", func() {
			It("should delete the VPA resource", func() {
				kapi = New(c, namespace, Values{Autoscaling: AutoscalingConfig{HVPAEnabled: true}})

				Expect(c.Create(ctx, verticalPodAutoscaler)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(verticalPodAutoscaler), verticalPodAutoscaler)).To(Succeed())
				Expect(kapi.Deploy(ctx)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(verticalPodAutoscaler), verticalPodAutoscaler)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: autoscalingv1beta2.SchemeGroupVersion.Group, Resource: "verticalpodautoscalers"}, verticalPodAutoscaler.Name)))
			})

			It("should successfully deploy the VPA resource", func() {
				autoscalingConfig := AutoscalingConfig{HVPAEnabled: false}
				kapi = New(c, namespace, Values{Autoscaling: autoscalingConfig})

				Expect(c.Get(ctx, client.ObjectKeyFromObject(verticalPodAutoscaler), verticalPodAutoscaler)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: autoscalingv1beta2.SchemeGroupVersion.Group, Resource: "verticalpodautoscalers"}, verticalPodAutoscaler.Name)))
				Expect(kapi.Deploy(ctx)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(verticalPodAutoscaler), verticalPodAutoscaler)).To(Succeed())
				Expect(verticalPodAutoscaler).To(DeepEqual(&autoscalingv1beta2.VerticalPodAutoscaler{
					TypeMeta: metav1.TypeMeta{
						APIVersion: autoscalingv1beta2.SchemeGroupVersion.String(),
						Kind:       "VerticalPodAutoscaler",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            verticalPodAutoscaler.Name,
						Namespace:       verticalPodAutoscaler.Namespace,
						ResourceVersion: "1",
					},
					Spec: autoscalingv1beta2.VerticalPodAutoscalerSpec{
						TargetRef: &autoscalingv1.CrossVersionObjectReference{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "kube-apiserver",
						},
						UpdatePolicy: &autoscalingv1beta2.PodUpdatePolicy{
							UpdateMode: &vpaUpdateMode,
						},
					},
				}))
			})
		})

		Describe("PodDisruptionBudget", func() {
			It("should successfully deploy the PDB resource", func() {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(podDisruptionBudget), podDisruptionBudget)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: policyv1beta1.SchemeGroupVersion.Group, Resource: "poddisruptionbudgets"}, podDisruptionBudget.Name)))
				Expect(kapi.Deploy(ctx)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(podDisruptionBudget), podDisruptionBudget)).To(Succeed())
				Expect(podDisruptionBudget).To(DeepEqual(&policyv1beta1.PodDisruptionBudget{
					TypeMeta: metav1.TypeMeta{
						APIVersion: policyv1beta1.SchemeGroupVersion.String(),
						Kind:       "PodDisruptionBudget",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            podDisruptionBudget.Name,
						Namespace:       podDisruptionBudget.Namespace,
						ResourceVersion: "1",
						Labels: map[string]string{
							"app":  "kubernetes",
							"role": "apiserver",
						},
					},
					Spec: policyv1beta1.PodDisruptionBudgetSpec{
						MaxUnavailable: intOrStrPtr(intstr.FromInt(1)),
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app":  "kubernetes",
								"role": "apiserver",
							},
						},
					},
				}))
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully delete all expected resources", func() {
			Expect(c.Create(ctx, horizontalPodAutoscaler)).To(Succeed())
			Expect(c.Create(ctx, verticalPodAutoscaler)).To(Succeed())
			Expect(c.Create(ctx, podDisruptionBudget)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(horizontalPodAutoscaler), horizontalPodAutoscaler)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(verticalPodAutoscaler), verticalPodAutoscaler)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(podDisruptionBudget), podDisruptionBudget)).To(Succeed())

			Expect(kapi.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(horizontalPodAutoscaler), horizontalPodAutoscaler)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: autoscalingv2beta1.SchemeGroupVersion.Group, Resource: "horizontalpodautoscalers"}, horizontalPodAutoscaler.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(verticalPodAutoscaler), verticalPodAutoscaler)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: autoscalingv1beta2.SchemeGroupVersion.Group, Resource: "verticalpodautoscalers"}, verticalPodAutoscaler.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(podDisruptionBudget), podDisruptionBudget)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: policyv1beta1.SchemeGroupVersion.Group, Resource: "poddisruptionbudgets"}, podDisruptionBudget.Name)))
		})
	})
})

func intOrStrPtr(intOrStr intstr.IntOrString) *intstr.IntOrString {
	return &intOrStr
}
