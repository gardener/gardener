// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package highavailabilityconfig_test

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("HighAvailabilityConfig tests", func() {
	var (
		namespace  *corev1.Namespace
		objectMeta metav1.ObjectMeta
	)

	BeforeEach(func() {
		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testIDPrefix + "-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8],
			},
		}

		objectMeta = metav1.ObjectMeta{
			Name:      testIDPrefix + "-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8],
			Namespace: namespace.Name,
		}
	})

	JustBeforeEach(func() {
		By("Create Namespace")
		Expect(testClient.Create(ctx, namespace)).To(Succeed())
		log.Info("Created Namespace", "namespaceName", namespace.Name)

		DeferCleanup(func() {
			By("Delete Namespace")
			Expect(testClient.Delete(ctx, namespace)).To(Succeed())
			log.Info("Deleted Namespace", "namespaceName", namespace.Name)
		})
	})

	Describe("Mutation of pod template spec", func() {
		var (
			deployment  *appsv1.Deployment
			statefulSet *appsv1.StatefulSet
			hpa         *autoscalingv2.HorizontalPodAutoscaler

			labels = map[string]string{"foo": "bar", "app": "foo"}
			zones  = []string{"a", "b", "c"}
		)

		BeforeEach(func() {
			deployment = &appsv1.Deployment{
				ObjectMeta: objectMeta,
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{MatchLabels: labels},
					Replicas: ptr.To[int32](1),
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
				ObjectMeta: objectMeta,
				Spec: appsv1.StatefulSetSpec{
					Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "foo"}},
					Replicas: ptr.To[int32](1),
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
		})

		JustBeforeEach(func() {
			By("Create HorizontalPodAutoscaler")
			Expect(testClient.Create(ctx, hpa)).To(Succeed())
			log.Info("Created HorizontalPodAutoscaler", "horizontalPodAutoscaler", client.ObjectKeyFromObject(hpa))

			DeferCleanup(func() {
				By("Delete HorizontalPodAutoscaler")
				Expect(testClient.Delete(ctx, hpa)).To(Succeed())
				log.Info("Deleted HorizontalPodAutoscaler", "horizontalPodAutoscaler", client.ObjectKeyFromObject(hpa))
			})
		})

		tests := func(
			getObj func() client.Object,
			getReplicas func() *int32,
			setReplicas func(*int32),
			getPodSpec func() corev1.PodSpec,
			setPodSpec func(func(*corev1.PodSpec)),
			matchLabelKeys []string,
		) {
			Context("when namespace is not labeled with consider=true", func() {
				It("should not mutate anything", func() {
					Expect(getReplicas()).To(PointTo(Equal(int32(1))))
					Expect(getPodSpec().Affinity).To(BeNil())
					Expect(getPodSpec().TopologySpreadConstraints).To(BeEmpty())
					Expect(getPodSpec().Tolerations).To(BeEmpty())
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
									setReplicas(ptr.To[int32](5))
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
								setReplicas(ptr.To[int32](0))
							})

							It("should not mutate the replicas", func() {
								Expect(getReplicas()).To(PointTo(Equal(int32(0))))
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
								Expect(getReplicas()).To(PointTo(Equal(int32(2))))
							})
						})

						Context("when failure tolerance type is empty", func() {
							BeforeEach(func() {
								metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigFailureToleranceType, "")
							})

							It("should mutate the replicas", func() {
								Expect(getReplicas()).To(PointTo(Equal(int32(1))))
							})
						})

						Context("when failure tolerance type is non-empty", func() {
							BeforeEach(func() {
								metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigFailureToleranceType, "foo")
							})

							It("should mutate the replicas", func() {
								Expect(getReplicas()).To(PointTo(Equal(int32(2))))
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
								Expect(getReplicas()).To(PointTo(Equal(int32(2))))
							})
						})

						Context("when failure tolerance type is empty", func() {
							BeforeEach(func() {
								metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigFailureToleranceType, "")
							})

							It("should mutate the replicas", func() {
								Expect(getReplicas()).To(PointTo(Equal(int32(2))))
							})
						})

						Context("when failure tolerance type is non-empty", func() {
							BeforeEach(func() {
								metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigFailureToleranceType, "foo")
							})

							It("should mutate the replicas", func() {
								Expect(getReplicas()).To(PointTo(Equal(int32(2))))
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

							Context("when correct affinity is already defined", func() {
								BeforeEach(func() {
									setPodSpec(func(spec *corev1.PodSpec) {
										spec.Affinity = &corev1.Affinity{
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
													},
												},
											},
										}
									})
								})

								It("should not re-add the same expressions", func() {
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
												},
											},
										},
									}))
								})
							})

							Context("when correct affinities are already defined", func() {
								BeforeEach(func() {
									setPodSpec(func(spec *corev1.PodSpec) {
										spec.Affinity = &corev1.Affinity{
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
															MatchExpressions: []corev1.NodeSelectorRequirement{{
																Key:      corev1.LabelTopologyZone,
																Operator: corev1.NodeSelectorOpIn,
																Values:   zones,
															}},
														},
													},
												},
											},
										}
									})
								})

								It("should not re-add the same expressions", func() {
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
					})
				})

				Context("topology spread constraints", func() {
					Context("when replicas are < 2", func() {
						It("should not add topology spread constraints", func() {
							Expect(getPodSpec().TopologySpreadConstraints).To(BeEmpty())
						})
					})

					Context("when replicas are >= 2", func() {
						var labelSelector *metav1.LabelSelector

						BeforeEach(func() {
							setReplicas(ptr.To[int32](2))

							switch o := getObj().(type) {
							case *appsv1.Deployment:
								labelSelector = &metav1.LabelSelector{MatchLabels: labels}
							case *appsv1.StatefulSet:
								labelSelector = o.Spec.Selector
							}
						})

						Context("when failure-tolerance-type is empty", func() {
							Context("when there are less than 2 zones", func() {
								It("should add topology spread constraints", func() {
									Expect(getPodSpec().TopologySpreadConstraints).To(ConsistOf(corev1.TopologySpreadConstraint{
										TopologyKey:       corev1.LabelHostname,
										MaxSkew:           1,
										WhenUnsatisfiable: corev1.ScheduleAnyway,
										LabelSelector:     labelSelector,
										MatchLabelKeys:    matchLabelKeys,
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
												LabelSelector:     labelSelector,
												MatchLabelKeys:    matchLabelKeys,
											},
											corev1.TopologySpreadConstraint{
												TopologyKey:       corev1.LabelTopologyZone,
												MaxSkew:           1,
												MinDomains:        ptr.To[int32](2),
												WhenUnsatisfiable: corev1.DoNotSchedule,
												LabelSelector:     labelSelector,
												MatchLabelKeys:    matchLabelKeys,
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
													LabelSelector:     labelSelector,
													MatchLabelKeys:    matchLabelKeys,
												},
												{
													TopologyKey:       corev1.LabelHostname,
													MaxSkew:           34,
													WhenUnsatisfiable: corev1.DoNotSchedule,
													LabelSelector:     labelSelector,
													MatchLabelKeys:    matchLabelKeys,
												},
												{
													TopologyKey:       corev1.LabelTopologyZone,
													MaxSkew:           56,
													WhenUnsatisfiable: corev1.ScheduleAnyway,
													LabelSelector:     labelSelector,
													MatchLabelKeys:    matchLabelKeys,
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
												LabelSelector:     labelSelector,
												MatchLabelKeys:    matchLabelKeys,
											},
											corev1.TopologySpreadConstraint{
												TopologyKey:       corev1.LabelHostname,
												MaxSkew:           1,
												WhenUnsatisfiable: corev1.ScheduleAnyway,
												LabelSelector:     labelSelector,
												MatchLabelKeys:    matchLabelKeys,
											},
											corev1.TopologySpreadConstraint{
												TopologyKey:       corev1.LabelTopologyZone,
												MaxSkew:           1,
												MinDomains:        ptr.To[int32](2),
												WhenUnsatisfiable: corev1.DoNotSchedule,
												LabelSelector:     labelSelector,
												MatchLabelKeys:    matchLabelKeys,
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
										MinDomains:        ptr.To[int32](2),
										MaxSkew:           1,
										WhenUnsatisfiable: corev1.DoNotSchedule,
										LabelSelector:     labelSelector,
										MatchLabelKeys:    matchLabelKeys,
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
											MinDomains:        ptr.To[int32](2),
											MaxSkew:           1,
											WhenUnsatisfiable: corev1.DoNotSchedule,
											LabelSelector:     labelSelector,
											MatchLabelKeys:    matchLabelKeys,
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
									var maxReplicas *int32
									if hpa != nil {
										maxReplicas = &hpa.Spec.MaxReplicas
									}

									minDomains := ptr.To(int32(len(zones)))
									if ptr.Deref(maxReplicas, *minDomains) < *minDomains {
										minDomains = maxReplicas
									}

									Expect(getPodSpec().TopologySpreadConstraints).To(ConsistOf(
										corev1.TopologySpreadConstraint{
											TopologyKey:       corev1.LabelHostname,
											MaxSkew:           1,
											WhenUnsatisfiable: corev1.ScheduleAnyway,
											LabelSelector:     labelSelector,
											MatchLabelKeys:    matchLabelKeys,
										},
										corev1.TopologySpreadConstraint{
											TopologyKey:       corev1.LabelTopologyZone,
											MaxSkew:           1,
											MinDomains:        minDomains,
											WhenUnsatisfiable: corev1.DoNotSchedule,
											LabelSelector:     labelSelector,
											MatchLabelKeys:    matchLabelKeys,
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
						})
					})
				})

				Context("tolerations", func() {
					Context("when no tolerations specified", func() {
						It("should default 'not-ready' and 'unreachable' seconds", func() {
							Expect(getPodSpec().Tolerations).To(ConsistOf(
								corev1.Toleration{
									Key:               "node.kubernetes.io/not-ready",
									Operator:          "Exists",
									Effect:            "NoExecute",
									TolerationSeconds: ptr.To(defaultNotReadyTolerationSeconds),
								},
								corev1.Toleration{
									Key:               "node.kubernetes.io/unreachable",
									Operator:          "Exists",
									Effect:            "NoExecute",
									TolerationSeconds: ptr.To(defaultUnreachableTolerationSeconds),
								},
							))
						})
					})

					Context("when some tolerations specified", func() {
						existingTolerations := []corev1.Toleration{
							{
								Key:      "foo",
								Operator: "Equal",
								Effect:   "NoSchedule",
								Value:    "bar",
							},
							{
								Key:               "foo",
								Operator:          "Exists",
								Effect:            "NoExecute",
								TolerationSeconds: ptr.To[int64](15),
							},
							{
								Key:      "node.kubernetes.io/not-ready",
								Operator: "Exists",
								Effect:   "NoSchedule",
							},
							{
								Key:      "node.kubernetes.io/unreachable",
								Operator: "Exists",
								Effect:   "NoSchedule",
							},
						}

						BeforeEach(func() {
							setPodSpec(func(spec *corev1.PodSpec) {
								spec.Tolerations = existingTolerations
							})
						})

						It("should default 'not-ready' and 'unreachable' seconds", func() {
							expectedTolerations := append(existingTolerations,
								corev1.Toleration{
									Key:               "node.kubernetes.io/not-ready",
									Operator:          "Exists",
									Effect:            "NoExecute",
									TolerationSeconds: ptr.To(defaultNotReadyTolerationSeconds),
								},
								corev1.Toleration{
									Key:               "node.kubernetes.io/unreachable",
									Operator:          "Exists",
									Effect:            "NoExecute",
									TolerationSeconds: ptr.To(defaultUnreachableTolerationSeconds),
								},
							)

							Expect(getPodSpec().Tolerations).To(ConsistOf(expectedTolerations))
						})
					})

					Context("when 'not-ready' toleration exists", func() {
						existingTolerations := []corev1.Toleration{{
							Key:               "node.kubernetes.io/not-ready",
							Operator:          "Exists",
							Effect:            "NoExecute",
							TolerationSeconds: ptr.To[int64](300),
						}}

						BeforeEach(func() {
							setPodSpec(func(spec *corev1.PodSpec) {
								spec.Tolerations = existingTolerations
							})
						})

						It("should default 'not-ready' and 'unreachable' seconds", func() {
							Expect(getPodSpec().Tolerations).To(ConsistOf(
								corev1.Toleration{
									Key:               "node.kubernetes.io/not-ready",
									Operator:          "Exists",
									Effect:            "NoExecute",
									TolerationSeconds: ptr.To[int64](300),
								},
								corev1.Toleration{
									Key:               "node.kubernetes.io/unreachable",
									Operator:          "Exists",
									Effect:            "NoExecute",
									TolerationSeconds: ptr.To(defaultUnreachableTolerationSeconds),
								},
							))
						})
					})

					Context("when 'unreachable' toleration exists", func() {
						existingTolerations := []corev1.Toleration{{
							Key:               "node.kubernetes.io/unreachable",
							Operator:          "Exists",
							Effect:            "NoExecute",
							TolerationSeconds: ptr.To[int64](300),
						}}

						BeforeEach(func() {
							setPodSpec(func(spec *corev1.PodSpec) {
								spec.Tolerations = existingTolerations
							})
						})

						It("should default 'not-ready' and 'unreachable' seconds", func() {
							Expect(getPodSpec().Tolerations).To(ConsistOf(
								corev1.Toleration{
									Key:               "node.kubernetes.io/not-ready",
									Operator:          "Exists",
									Effect:            "NoExecute",
									Value:             "",
									TolerationSeconds: ptr.To(defaultNotReadyTolerationSeconds),
								},
								corev1.Toleration{
									Key:               "node.kubernetes.io/unreachable",
									Operator:          "Exists",
									Effect:            "NoExecute",
									Value:             "",
									TolerationSeconds: ptr.To[int64](300),
								},
							))
						})
					})
				})
			})
		}

		Context("for deployments", func() {
			BeforeEach(func() {
				hpa.Spec.ScaleTargetRef.Kind = "Deployment"
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
				[]string{"pod-template-hash"},
			)
		})

		Context("for statefulsets", func() {
			BeforeEach(func() {
				hpa.Spec.ScaleTargetRef.Kind = "StatefulSet"
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
				nil,
			)
		})
	})

	Describe("Mutation of scaling objects", func() {
		var scalingObject client.Object

		JustBeforeEach(func() {
			By("Create scaling object")
			Expect(testClient.Create(ctx, scalingObject)).To(Succeed())
		})

		tests := func(
			getObjectMeta func() *metav1.ObjectMeta,
			getMinReplicas func() *int32,
			setMinReplicas func(*int32),
			getMaxReplicas func() int32,
			setMaxReplicas func(int32),
		) {
			var minReplicas, maxReplicas int32

			BeforeEach(func() {
				minReplicas = 1
				maxReplicas = 2

				setMinReplicas(&minReplicas)
				setMaxReplicas(maxReplicas)
			})

			Context("when namespace is not labeled with consider=true", func() {
				BeforeEach(func() {
					metav1.SetMetaDataLabel(getObjectMeta(), resourcesv1alpha1.HighAvailabilityConfigType, resourcesv1alpha1.HighAvailabilityConfigTypeServer)
				})

				It("should not modify replica counts", func() {
					Expect(getMinReplicas()).To(PointTo(Equal(minReplicas)))
					Expect(getMaxReplicas()).To(Equal(maxReplicas))
				})
			})

			Context("when namespace is labeled with consider=true", func() {
				BeforeEach(func() {
					metav1.SetMetaDataLabel(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigConsider, "true")
				})

				Context("when failureToleranceType is not set", func() {
					It("should not modify replica counts", func() {
						Expect(getMinReplicas()).To(PointTo(Equal(minReplicas)))
						Expect(getMaxReplicas()).To(Equal(maxReplicas))
					})
				})

				Context("when failureToleranceType and component type are set", func() {
					BeforeEach(func() {
						metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigFailureToleranceType, "zone")
						metav1.SetMetaDataLabel(getObjectMeta(), resourcesv1alpha1.HighAvailabilityConfigType, resourcesv1alpha1.HighAvailabilityConfigTypeController)
					})

					It("should match expected replica count configuration", func() {
						Expect(getMinReplicas()).To(PointTo(Equal(int32(2))))
						Expect(getMaxReplicas()).To(Equal(maxReplicas))
					})

					Context("when minReplicas <= maxReplicas", func() {
						BeforeEach(func() {
							setMaxReplicas(minReplicas)
						})

						It("should set maxReplicas to minReplicas", func() {
							Expect(getMinReplicas()).To(PointTo(Equal(int32(2))))
							Expect(getMaxReplicas()).To(Equal(int32(2)))
						})
					})

					Context("when replica count is overwritten by annotation", func() {
						var newReplicaCount int32

						BeforeEach(func() {
							newReplicaCount = 12
							metav1.SetMetaDataAnnotation(getObjectMeta(), resourcesv1alpha1.HighAvailabilityConfigReplicas, fmt.Sprintf("%d", newReplicaCount))
						})

						It("should set replica counts to value in annotation", func() {
							Expect(getMinReplicas()).To(PointTo(Equal(newReplicaCount)))
							Expect(getMaxReplicas()).To(Equal(newReplicaCount))
						})
					})

					Context("when current replica counts are higher than calculated ones", func() {
						var newMinReplicas, newMaxReplicas int32

						BeforeEach(func() {
							newMinReplicas = 12
							newMaxReplicas = 16

							setMinReplicas(&newMinReplicas)
							setMaxReplicas(newMaxReplicas)
						})

						It("should not modify replica counts", func() {
							Expect(getMinReplicas()).To(PointTo(Equal(newMinReplicas)))
							Expect(getMaxReplicas()).To(Equal(newMaxReplicas))
						})
					})
				})
			})
		}

		Context("for HPAs", func() {
			Context("with version v2", func() {
				var hpa *autoscalingv2.HorizontalPodAutoscaler

				BeforeEach(func() {
					hpa = &autoscalingv2.HorizontalPodAutoscaler{
						ObjectMeta: objectMeta,
						Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
							ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
								Kind: "someKind",
								Name: "someName",
							},
						},
					}

					scalingObject = hpa
				})

				tests(
					func() *metav1.ObjectMeta { return &hpa.ObjectMeta },
					func() *int32 { return hpa.Spec.MinReplicas },
					func(n *int32) { hpa.Spec.MinReplicas = n },
					func() int32 { return hpa.Spec.MaxReplicas },
					func(n int32) { hpa.Spec.MaxReplicas = n },
				)
			})
		})
	})
})
