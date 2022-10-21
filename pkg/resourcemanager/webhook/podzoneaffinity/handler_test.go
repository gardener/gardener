// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package podzoneaffinity_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/resourcemanager/webhook/podzoneaffinity"
)

var _ = Describe("Handler", func() {
	var (
		ctx        = context.Background()
		fakeClient client.Client
		log        logr.Logger
		handler    *Handler

		namespace *corev1.Namespace
		pod       *corev1.Pod

		namespaceName = "shoot--foo--bar"
		zone          = "zone-a"
	)

	BeforeEach(func() {
		ctx = admission.NewContextWithRequest(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Namespace: namespaceName}})
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		handler = &Handler{Logger: log, Client: fakeClient}

		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespaceName,
			},
		}
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-0",
				Namespace: namespace.Name,
			},
			Spec: corev1.PodSpec{},
		}

		Expect(fakeClient.Create(ctx, namespace)).To(Succeed())
	})

	Describe("#Default", func() {
		Context("when namespace is not labeled with zone enforcement", func() {
			It("should add the zone pod affinity if affinity is not set", func() {
				pod.Spec.Affinity = nil

				Expect(handler.Default(ctx, pod)).To(Succeed())
				Expect(pod.Spec.Affinity.NodeAffinity).To(BeNil())
				Expect(pod.Spec.Affinity.PodAffinity).To(Equal(&corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
						LabelSelector: &metav1.LabelSelector{},
						TopologyKey:   corev1.LabelTopologyZone,
					}},
				}))
				Expect(pod.Spec.Affinity.PodAntiAffinity).To(BeNil())
			})

			It("should add the zone pod affinity if another pod affinity is set", func() {
				pod.Spec.Affinity = &corev1.Affinity{
					PodAffinity: &corev1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								TopologyKey: corev1.LabelHostname,
							},
						},
					},
				}

				Expect(handler.Default(ctx, pod)).To(Succeed())
				Expect(pod.Spec.Affinity.NodeAffinity).To(BeNil())
				Expect(pod.Spec.Affinity.PodAffinity).To(Equal(&corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							TopologyKey: corev1.LabelHostname,
						},
						{
							LabelSelector: &metav1.LabelSelector{},
							TopologyKey:   corev1.LabelTopologyZone,
						},
					},
				}))
				Expect(pod.Spec.Affinity.PodAntiAffinity).To(BeNil())
			})
		})

		Context("when namespace is labeled with zone enforcement", func() {
			BeforeEach(func() {
				namespace.Labels = map[string]string{"control-plane.shoot.gardener.cloud/enforce-zone": zone}
				Expect(fakeClient.Update(ctx, namespace)).To(Succeed())
			})

			It("should add the zone pod affinity for a specific zone if affinity is not set", func() {
				Expect(handler.Default(ctx, pod)).To(Succeed())
				Expect(pod.Spec.Affinity.NodeAffinity).To(Equal(&corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{{
							MatchExpressions: []corev1.NodeSelectorRequirement{{
								Key:      corev1.LabelTopologyZone,
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{zone},
							}},
						}},
					},
				}))
				Expect(pod.Spec.Affinity.PodAffinity).To(Equal(&corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
						LabelSelector: &metav1.LabelSelector{},
						TopologyKey:   corev1.LabelTopologyZone,
					}},
				}))
				Expect(pod.Spec.Affinity.PodAntiAffinity).To(BeNil())
			})

			It("should add the zone pod affinity for a specific zone if another node affinity is set", func() {
				pod.Spec.Affinity = &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      corev1.LabelHostname,
											Operator: corev1.NodeSelectorOpNotIn,
											Values:   []string{"foo", "bar"},
										},
									},
								},
							},
						},
					},
					PodAffinity: &corev1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								TopologyKey:   corev1.LabelTopologyZone,
								LabelSelector: &metav1.LabelSelector{},
							},
						},
					},
				}

				Expect(handler.Default(ctx, pod)).To(Succeed())
				Expect(pod.Spec.Affinity.NodeAffinity).To(Equal(&corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      corev1.LabelHostname,
										Operator: corev1.NodeSelectorOpNotIn,
										Values:   []string{"foo", "bar"},
									},
								},
							},
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      corev1.LabelTopologyZone,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{zone},
									},
								},
							},
						},
					},
				}))
				Expect(pod.Spec.Affinity.PodAffinity).To(Equal(&corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							TopologyKey:   corev1.LabelTopologyZone,
							LabelSelector: &metav1.LabelSelector{},
						},
					},
				}))
				Expect(pod.Spec.Affinity.PodAntiAffinity).To(BeNil())
			})

			It("should not change anything as required affinities are set", func() {
				pod.Spec.Affinity = &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      corev1.LabelTopologyZone,
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{zone},
										},
									},
								},
							},
						},
					},
					PodAffinity: &corev1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								TopologyKey:   corev1.LabelTopologyZone,
								LabelSelector: &metav1.LabelSelector{},
							},
						},
					},
				}

				Expect(handler.Default(ctx, pod)).To(Succeed())
				Expect(pod.Spec.Affinity.NodeAffinity).To(Equal(&corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      corev1.LabelTopologyZone,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{zone},
									},
								},
							},
						},
					},
				}))
				Expect(pod.Spec.Affinity.PodAffinity).To(Equal(&corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							TopologyKey:   corev1.LabelTopologyZone,
							LabelSelector: &metav1.LabelSelector{},
						},
					},
				}))
				Expect(pod.Spec.Affinity.PodAntiAffinity).To(BeNil())
			})

			It("should remove conflicting terms", func() {
				pod.Spec.Affinity = &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key: corev1.LabelTopologyZone,
										},
									},
								},
							},
						},
					},
					PodAffinity: &corev1.PodAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								TopologyKey: corev1.LabelTopologyZone,
								Namespaces:  []string{"default"},
							},
						},
					},
					PodAntiAffinity: &corev1.PodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								TopologyKey:   corev1.LabelTopologyZone,
								LabelSelector: &metav1.LabelSelector{},
							},
							{
								TopologyKey: corev1.LabelHostname,
							},
						},
					},
				}

				Expect(handler.Default(ctx, pod)).To(Succeed())
				Expect(pod.Spec.Affinity.NodeAffinity).To(Equal(&corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      corev1.LabelTopologyZone,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{zone},
									},
								},
							},
						},
					},
				}))
				Expect(pod.Spec.Affinity.PodAffinity).To(Equal(&corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							LabelSelector: &metav1.LabelSelector{},
							TopologyKey:   corev1.LabelTopologyZone,
						},
					},
				}))
				Expect(pod.Spec.Affinity.PodAntiAffinity).To(Equal(&corev1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							TopologyKey: corev1.LabelHostname,
						},
					},
				}))
			})
		})
	})
})
