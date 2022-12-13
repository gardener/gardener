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

package highavailabilityconfig_test

import (
	"strings"

	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("HighAvailabilityConfig tests", func() {
	var (
		namespace   *corev1.Namespace
		deployment  *appsv1.Deployment
		statefulSet *appsv1.StatefulSet
		hpa         *autoscalingv2.HorizontalPodAutoscaler
		hvpa        *hvpav1alpha1.Hvpa

		labels = map[string]string{"foo": "bar"}
		zones  = []string{"a", "b", "c"}
	)

	BeforeEach(func() {
		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testIDPrefix + "-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8],
			},
		}

		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testIDPrefix + "-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8],
				Namespace: namespace.Name,
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: labels},
				Replicas: pointer.Int32(1),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: labels,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "foo-container",
							Image: "foo",
						}},
					},
				},
			},
		}

		statefulSet = &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testIDPrefix + "-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8],
				Namespace: namespace.Name,
			},
			Spec: appsv1.StatefulSetSpec{
				Selector: &metav1.LabelSelector{MatchLabels: labels},
				Replicas: pointer.Int32(1),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: labels,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "foo-container",
							Image: "foo",
						}},
					},
				},
			},
		}

		hpa = &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testIDPrefix + "-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8],
				Namespace: namespace.Name,
			},
			Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
				MaxReplicas: 5,
				ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "something",
					Name:       "something",
				},
			},
		}

		hvpa = &hvpav1alpha1.Hvpa{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testIDPrefix + "-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8],
				Namespace: namespace.Name,
			},
			Spec: hvpav1alpha1.HvpaSpec{
				Hpa: hvpav1alpha1.HpaSpec{
					Deploy: true,
				},
				TargetRef: &autoscalingv2beta1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "something",
					Name:       "something",
				},
			},
		}
	})

	JustBeforeEach(func() {
		By("Create Namespace")
		Expect(testClient.Create(ctx, namespace)).To(Succeed())
		log.Info("Created Namespace", "namespaceName", namespace.Name)

		By("Create HorizontalPodAutoscaler")
		Expect(testClient.Create(ctx, hpa)).To(Succeed())
		log.Info("Created HorizontalPodAutoscaler", "horizontalPodAutoscaler", client.ObjectKeyFromObject(hpa))

		By("Create HVPA")
		Expect(testClient.Create(ctx, hvpa)).To(Succeed())
		log.Info("Created HVPA", "hvpa", client.ObjectKeyFromObject(hvpa))

		DeferCleanup(func() {
			By("Delete HVPA")
			Expect(testClient.Delete(ctx, hvpa)).To(Succeed())
			log.Info("Deleted HorizontalPodAutoscaler", "hvpa", client.ObjectKeyFromObject(hvpa))

			By("Delete HorizontalPodAutoscaler")
			Expect(testClient.Delete(ctx, hpa)).To(Succeed())
			log.Info("Deleted HorizontalPodAutoscaler", "horizontalPodAutoscaler", client.ObjectKeyFromObject(hpa))

			By("Delete Namespace")
			Expect(testClient.Delete(ctx, namespace)).To(Succeed())
			log.Info("Deleted Namespace", "namespaceName", namespace.Name)
		})
	})

	tests := func(
		getObj func() client.Object,
		getReplicas func() *int32,
		setReplicas func(*int32),
		getPodSpec func() corev1.PodSpec,
		setPodSpec func(func(*corev1.PodSpec)),
	) {
		Context("when namespace is not labeled with consider=true", func() {
			It("should not mutate anything", func() {
				Expect(getReplicas()).To(PointTo(Equal(int32(1))))
				Expect(getPodSpec().Affinity).To(BeNil())
				Expect(getPodSpec().TopologySpreadConstraints).To(BeEmpty())
			})
		})

		Context("when namespace is labeled with consider=true", func() {
			BeforeEach(func() {
				metav1.SetMetaDataLabel(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigConsider, "true")
			})

			Context("replicas", func() {
				Context("when resource does not have type label", func() {
					It("should not mutate the replicas", func() {
						Expect(getReplicas()).To(PointTo(Equal(int32(1))))
					})
				})

				horizontallyScaledTests := func(expectedReplicas int32) {
					Context("via HPA", func() {
						BeforeEach(func() {
							hpa.Spec.ScaleTargetRef.Name = getObj().GetName()
						})

						Context("current replicas lower than the computed replicas", func() {
							It("should mutate the replicas", func() {
								Expect(getReplicas()).To(PointTo(Equal(expectedReplicas)))
							})
						})

						Context("current replicas are higher than the computed replicas", func() {
							BeforeEach(func() {
								setReplicas(pointer.Int32(5))
							})

							It("should not mutate the replicas", func() {
								Expect(getReplicas()).To(PointTo(Equal(int32(5))))
							})
						})
					})

					Context("via HVPA", func() {
						BeforeEach(func() {
							hvpa.Spec.TargetRef.Name = getObj().GetName()
						})

						Context("current replicas lower than the computed replicas", func() {
							It("should mutate the replicas", func() {
								Expect(getReplicas()).To(PointTo(Equal(expectedReplicas)))
							})
						})

						Context("current replicas are higher than the computed replicas", func() {
							BeforeEach(func() {
								setReplicas(pointer.Int32(5))
							})

							It("should not mutate the replicas", func() {
								Expect(getReplicas()).To(PointTo(Equal(int32(5))))
							})
						})
					})
				}

				specialCasesTests := func(expectedReplicas int32) {
					Context("when resource is horizontally scaled", func() {
						horizontallyScaledTests(expectedReplicas)
					})

					Context("when replicas are 0", func() {
						BeforeEach(func() {
							setReplicas(pointer.Int32(0))
						})

						It("should not mutate the replicas", func() {
							Expect(getReplicas()).To(Equal(pointer.Int32(0)))
						})
					})

					Context("when replicas are overwritten", func() {
						BeforeEach(func() {
							getObj().SetAnnotations(utils.MergeStringMaps(getObj().GetAnnotations(), map[string]string{
								resourcesv1alpha1.HighAvailabilityConfigReplicas: "4",
							}))
						})

						It("should use the overwritten value", func() {
							Expect(getReplicas()).To(PointTo(Equal(int32(4))))
						})
					})
				}

				Context("when resource is of type 'controller'", func() {
					BeforeEach(func() {
						getObj().SetLabels(utils.MergeStringMaps(getObj().GetLabels(), map[string]string{
							resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeController,
						}))
					})

					Context("when failure tolerance type is nil", func() {
						It("should mutate the replicas", func() {
							Expect(getReplicas()).To(Equal(pointer.Int32(2)))
						})
					})

					Context("when failure tolerance type is empty", func() {
						BeforeEach(func() {
							metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigFailureToleranceType, "")
						})

						It("should mutate the replicas", func() {
							Expect(getReplicas()).To(Equal(pointer.Int32(1)))
						})
					})

					Context("when failure tolerance type is non-empty", func() {
						BeforeEach(func() {
							metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigFailureToleranceType, "foo")
						})

						It("should mutate the replicas", func() {
							Expect(getReplicas()).To(Equal(pointer.Int32(2)))
						})
					})

					Context("special cases", func() {
						specialCasesTests(2)
					})
				})

				Context("when resource is of type 'server'", func() {
					BeforeEach(func() {
						getObj().SetLabels(utils.MergeStringMaps(getObj().GetLabels(), map[string]string{
							resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeServer,
						}))
					})

					Context("when failure tolerance type is nil", func() {
						It("should mutate the replicas", func() {
							Expect(getReplicas()).To(Equal(pointer.Int32(2)))
						})
					})

					Context("when failure tolerance type is empty", func() {
						BeforeEach(func() {
							metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigFailureToleranceType, "")
						})

						It("should mutate the replicas", func() {
							Expect(getReplicas()).To(Equal(pointer.Int32(2)))
						})
					})

					Context("when failure tolerance type is non-empty", func() {
						BeforeEach(func() {
							metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigFailureToleranceType, "foo")
						})

						It("should mutate the replicas", func() {
							Expect(getReplicas()).To(Equal(pointer.Int32(2)))
						})
					})

					Context("special cases", func() {
						specialCasesTests(2)
					})
				})
			})

			Context("affinity", func() {
				Context("when namespace is not annotated with neither failure-tolerance-type nor zones", func() {
					It("should not mutate the node affinity", func() {
						Expect(getPodSpec().Affinity).To(BeNil())
					})
				})

				Context("when namespace is annotated with failure-tolerance-type but not with zones", func() {
					BeforeEach(func() {
						metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigFailureToleranceType, "foo")
					})

					It("should not mutate the node affinity", func() {
						Expect(getPodSpec().Affinity).To(BeNil())
					})
				})

				Context("when namespace is annotated with failure-tolerance-type but empty zones", func() {
					BeforeEach(func() {
						metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigZones, "")
						metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigFailureToleranceType, "foo")
					})

					It("should not mutate the node affinity", func() {
						Expect(getPodSpec().Affinity).To(BeNil())
					})
				})

				Context("when namespace is annotated with failure-tolerance-type and non-empty zones", func() {
					BeforeEach(func() {
						metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigZones, strings.Join(zones, ","))
						metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigFailureToleranceType, "foo")
					})

					Context("when there are no existing node affinities in spec", func() {
						It("should add a node affinity", func() {
							Expect(getPodSpec().Affinity).To(Equal(&corev1.Affinity{
								NodeAffinity: &corev1.NodeAffinity{
									RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
										NodeSelectorTerms: []corev1.NodeSelectorTerm{{
											MatchExpressions: []corev1.NodeSelectorRequirement{{
												Key:      corev1.LabelTopologyZone,
												Operator: corev1.NodeSelectorOpIn,
												Values:   zones,
											}},
										}},
									},
								},
							}))
						})
					})

					Context("when there are existing node affinities in spec", func() {
						BeforeEach(func() {
							setPodSpec(func(spec *corev1.PodSpec) {
								spec.Affinity = &corev1.Affinity{
									NodeAffinity: &corev1.NodeAffinity{
										RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
											NodeSelectorTerms: []corev1.NodeSelectorTerm{
												{
													MatchExpressions: []corev1.NodeSelectorRequirement{{
														Key:      corev1.LabelHostname,
														Operator: corev1.NodeSelectorOpExists,
													}},
												},
												{
													MatchExpressions: []corev1.NodeSelectorRequirement{{
														Key:      corev1.LabelTopologyZone,
														Operator: corev1.NodeSelectorOpNotIn,
														Values:   []string{"some", "other", "zones"},
													}},
												},
												{
													MatchExpressions: []corev1.NodeSelectorRequirement{{
														Key:      "foo",
														Operator: corev1.NodeSelectorOpNotIn,
														Values:   []string{"bar"},
													}},
												},
											},
										},
									},
								}
							})
						})

						It("should add a node affinity", func() {
							Expect(getPodSpec().Affinity).To(Equal(&corev1.Affinity{
								NodeAffinity: &corev1.NodeAffinity{
									RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
										NodeSelectorTerms: []corev1.NodeSelectorTerm{
											{
												MatchExpressions: []corev1.NodeSelectorRequirement{
													{
														Key:      corev1.LabelHostname,
														Operator: corev1.NodeSelectorOpExists,
													},
													{
														Key:      corev1.LabelTopologyZone,
														Operator: corev1.NodeSelectorOpIn,
														Values:   zones,
													},
												},
											},
											{
												MatchExpressions: []corev1.NodeSelectorRequirement{
													{
														Key:      "foo",
														Operator: corev1.NodeSelectorOpNotIn,
														Values:   []string{"bar"},
													},
													{
														Key:      corev1.LabelTopologyZone,
														Operator: corev1.NodeSelectorOpIn,
														Values:   zones,
													},
												},
											},
										},
									},
								},
							}))
						})
					})
				})

				Context("when namespace is annotated with zone pinning and non-empty zones, but not failure-tolerance-type", func() {
					BeforeEach(func() {
						metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigZones, strings.Join(zones, ","))
						metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigZonePinning, "true")
					})

					Context("when there are no existing node affinities in spec", func() {
						It("should add a node affinity", func() {
							Expect(getPodSpec().Affinity).To(Equal(&corev1.Affinity{
								NodeAffinity: &corev1.NodeAffinity{
									RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
										NodeSelectorTerms: []corev1.NodeSelectorTerm{{
											MatchExpressions: []corev1.NodeSelectorRequirement{{
												Key:      corev1.LabelTopologyZone,
												Operator: corev1.NodeSelectorOpIn,
												Values:   zones,
											}},
										}},
									},
								},
							}))
						})
					})
				})

				Context("when namespace is annotated with zones, but neither with zone-pinning nor failure-tolerance-type", func() {
					BeforeEach(func() {
						metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigZones, strings.Join(zones, ","))
					})

					It("should not mutate the node affinity", func() {
						Expect(getPodSpec().Affinity).To(BeNil())
					})
				})
			})

			Context("topology spread constraints", func() {
				Context("when replicas are < 2", func() {
					It("should not add topology spread constraints", func() {
						Expect(getPodSpec().TopologySpreadConstraints).To(BeEmpty())
					})
				})

				Context("when replicas are >= 2", func() {
					BeforeEach(func() {
						setReplicas(pointer.Int32(2))
					})

					Context("when failure-tolerance-type is empty", func() {
						Context("when there are less than 2 zones", func() {
							It("should add topology spread constraints", func() {
								Expect(getPodSpec().TopologySpreadConstraints).To(ConsistOf(corev1.TopologySpreadConstraint{
									TopologyKey:       corev1.LabelHostname,
									MaxSkew:           1,
									WhenUnsatisfiable: corev1.ScheduleAnyway,
									LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
								}))
							})
						})

						Context("when there are at least 2 zones", func() {
							BeforeEach(func() {
								metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigZones, strings.Join(zones, ","))
							})

							Context("when there are no existing constraints in spec", func() {
								It("should add topology spread constraints", func() {
									Expect(getPodSpec().TopologySpreadConstraints).To(ConsistOf(
										corev1.TopologySpreadConstraint{
											TopologyKey:       corev1.LabelHostname,
											MaxSkew:           1,
											WhenUnsatisfiable: corev1.ScheduleAnyway,
											LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
										},
										corev1.TopologySpreadConstraint{
											TopologyKey:       corev1.LabelTopologyZone,
											MaxSkew:           1,
											WhenUnsatisfiable: corev1.DoNotSchedule,
											LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
										},
									))
								})
							})

							Context("when there are existing constraints in spec", func() {
								BeforeEach(func() {
									setPodSpec(func(spec *corev1.PodSpec) {
										spec.TopologySpreadConstraints = []corev1.TopologySpreadConstraint{
											{
												TopologyKey:       "some-key",
												MaxSkew:           12,
												WhenUnsatisfiable: corev1.DoNotSchedule,
												LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
											},
											{
												TopologyKey:       corev1.LabelHostname,
												MaxSkew:           34,
												WhenUnsatisfiable: corev1.DoNotSchedule,
												LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
											},
											{
												TopologyKey:       corev1.LabelTopologyZone,
												MaxSkew:           56,
												WhenUnsatisfiable: corev1.ScheduleAnyway,
												LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
											},
										}
									})
								})

								It("should add topology spread constraints", func() {
									Expect(getPodSpec().TopologySpreadConstraints).To(ConsistOf(
										corev1.TopologySpreadConstraint{
											TopologyKey:       "some-key",
											MaxSkew:           12,
											WhenUnsatisfiable: corev1.DoNotSchedule,
											LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
										},
										corev1.TopologySpreadConstraint{
											TopologyKey:       corev1.LabelHostname,
											MaxSkew:           1,
											WhenUnsatisfiable: corev1.ScheduleAnyway,
											LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
										},
										corev1.TopologySpreadConstraint{
											TopologyKey:       corev1.LabelTopologyZone,
											MaxSkew:           1,
											WhenUnsatisfiable: corev1.DoNotSchedule,
											LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
										},
									))
								})
							})
						})
					})

					Context("when failure-tolerance-type is non-empty", func() {
						BeforeEach(func() {
							metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigFailureToleranceType, "foo")
						})

						Context("when there are less than 2 zones", func() {
							It("should add topology spread constraints", func() {
								Expect(getPodSpec().TopologySpreadConstraints).To(ConsistOf(corev1.TopologySpreadConstraint{
									TopologyKey:       corev1.LabelHostname,
									MaxSkew:           1,
									WhenUnsatisfiable: corev1.DoNotSchedule,
									LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
								}))
							})
						})

						Context("when there are at least 2 zones", func() {
							BeforeEach(func() {
								metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigZones, strings.Join(zones, ","))
							})

							It("should add topology spread constraints", func() {
								Expect(getPodSpec().TopologySpreadConstraints).To(ConsistOf(
									corev1.TopologySpreadConstraint{
										TopologyKey:       corev1.LabelHostname,
										MaxSkew:           1,
										WhenUnsatisfiable: corev1.DoNotSchedule,
										LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
									},
								))
							})
						})
					})

					Context("when max replicas are at least twice the number of zones", func() {
						BeforeEach(func() {
							metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigZones, strings.Join(zones, ","))
						})

						test := func() {
							It("should add topology spread constraints", func() {
								Expect(getPodSpec().TopologySpreadConstraints).To(ConsistOf(
									corev1.TopologySpreadConstraint{
										TopologyKey:       corev1.LabelHostname,
										MaxSkew:           1,
										WhenUnsatisfiable: corev1.ScheduleAnyway,
										LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
									},
									corev1.TopologySpreadConstraint{
										TopologyKey:       corev1.LabelTopologyZone,
										MaxSkew:           2,
										WhenUnsatisfiable: corev1.DoNotSchedule,
										LabelSelector:     &metav1.LabelSelector{MatchLabels: labels},
									},
								))
							})
						}

						Context("scaled w/ HPA", func() {
							BeforeEach(func() {
								hpa.Spec.ScaleTargetRef.Name = getObj().GetName()
								hpa.Spec.MaxReplicas = int32(2 * len(zones))
							})

							test()
						})

						Context("scaled w/ HVPA", func() {
							BeforeEach(func() {
								hvpa.Spec.TargetRef.Name = getObj().GetName()
								hvpa.Spec.Hpa.Template.Spec.MaxReplicas = int32(2 * len(zones))
							})

							test()
						})
					})
				})
			})
		})
	}

	Context("for deployments", func() {
		BeforeEach(func() {
			hpa.Spec.ScaleTargetRef.Kind = "Deployment"
			hvpa.Spec.TargetRef.Kind = "Deployment"
		})

		JustBeforeEach(func() {
			By("Create Deployment")
			Expect(testClient.Create(ctx, deployment)).To(Succeed())
		})

		tests(
			func() client.Object { return deployment },
			func() *int32 { return deployment.Spec.Replicas },
			func(replicas *int32) { deployment.Spec.Replicas = replicas },
			func() corev1.PodSpec { return deployment.Spec.Template.Spec },
			func(mutate func(spec *corev1.PodSpec)) { mutate(&deployment.Spec.Template.Spec) },
		)
	})

	Context("for statefulsets", func() {
		BeforeEach(func() {
			hpa.Spec.ScaleTargetRef.Kind = "StatefulSet"
			hvpa.Spec.TargetRef.Kind = "StatefulSet"
		})

		JustBeforeEach(func() {
			By("Create StatefulSet")
			Expect(testClient.Create(ctx, statefulSet)).To(Succeed())
		})

		tests(
			func() client.Object { return statefulSet },
			func() *int32 { return statefulSet.Spec.Replicas },
			func(replicas *int32) { statefulSet.Spec.Replicas = replicas },
			func() corev1.PodSpec { return statefulSet.Spec.Template.Spec },
			func(mutate func(spec *corev1.PodSpec)) { mutate(&statefulSet.Spec.Template.Spec) },
		)
	})
})
