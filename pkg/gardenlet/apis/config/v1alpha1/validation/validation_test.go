// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1/validation"
)

var _ = Describe("GardenletConfiguration", func() {
	var (
		cfg *gardenletconfigv1alpha1.GardenletConfiguration

		deletionGracePeriodHours = 1
		concurrentSyncs          = 20
	)

	BeforeEach(func() {
		cfg = &gardenletconfigv1alpha1.GardenletConfiguration{
			Controllers: &gardenletconfigv1alpha1.GardenletControllerConfiguration{
				BackupEntry: &gardenletconfigv1alpha1.BackupEntryControllerConfiguration{
					DeletionGracePeriodHours:         &deletionGracePeriodHours,
					DeletionGracePeriodShootPurposes: []gardencorev1beta1.ShootPurpose{gardencorev1beta1.ShootPurposeDevelopment},
				},
				Bastion: &gardenletconfigv1alpha1.BastionControllerConfiguration{
					ConcurrentSyncs: &concurrentSyncs,
				},
				Shoot: &gardenletconfigv1alpha1.ShootControllerConfiguration{
					ConcurrentSyncs:      &concurrentSyncs,
					ProgressReportPeriod: &metav1.Duration{Duration: time.Hour},
					SyncPeriod:           &metav1.Duration{Duration: time.Hour},
					RetryDuration:        &metav1.Duration{Duration: time.Hour},
					DNSEntryTTLSeconds:   ptr.To[int64](120),
				},
				ShootCare: &gardenletconfigv1alpha1.ShootCareControllerConfiguration{
					ConcurrentSyncs:                     &concurrentSyncs,
					SyncPeriod:                          &metav1.Duration{Duration: time.Hour},
					StaleExtensionHealthChecks:          &gardenletconfigv1alpha1.StaleExtensionHealthChecks{Threshold: &metav1.Duration{Duration: time.Hour}},
					ManagedResourceProgressingThreshold: &metav1.Duration{Duration: time.Hour},
					ConditionThresholds:                 []gardenletconfigv1alpha1.ConditionThreshold{{Duration: metav1.Duration{Duration: time.Hour}}},
				},
				ManagedSeed: &gardenletconfigv1alpha1.ManagedSeedControllerConfiguration{
					ConcurrentSyncs:  &concurrentSyncs,
					SyncPeriod:       &metav1.Duration{Duration: 1 * time.Hour},
					WaitSyncPeriod:   &metav1.Duration{Duration: 15 * time.Second},
					SyncJitterPeriod: &metav1.Duration{Duration: 5 * time.Minute},
				},
			},
			FeatureGates: map[string]bool{},
			SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
				SeedTemplate: gardencorev1beta1.SeedTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"foo": "bar",
						},
					},
					Spec: gardencorev1beta1.SeedSpec{
						DNS: gardencorev1beta1.SeedDNS{
							Provider: &gardencorev1beta1.SeedDNSProvider{
								Type: "foo",
								SecretRef: corev1.SecretReference{
									Name:      "secret",
									Namespace: "namespace",
								},
							},
						},
						Ingress: &gardencorev1beta1.Ingress{
							Domain: "ingress.test.example.com",
							Controller: gardencorev1beta1.IngressController{
								Kind: "nginx",
							},
						},
						Networks: gardencorev1beta1.SeedNetworks{
							Pods:     "100.96.0.0/11",
							Services: "100.64.0.0/13",
						},
						Provider: gardencorev1beta1.SeedProvider{
							Type:   "foo",
							Region: "some-region",
						},
					},
				},
			},
			Resources: &gardenletconfigv1alpha1.ResourcesConfiguration{
				Capacity: corev1.ResourceList{
					"foo": resource.MustParse("42"),
					"bar": resource.MustParse("13"),
				},
				Reserved: corev1.ResourceList{
					"foo": resource.MustParse("7"),
				},
			},
		}
	})

	Describe("#ValidateGardenletConfiguration", func() {
		It("should allow valid configurations", func() {
			errorList := ValidateGardenletConfiguration(cfg, nil, false)

			Expect(errorList).To(BeEmpty())
		})

		Context("client connection configuration", func() {
			var (
				clientConnection *componentbaseconfigv1alpha1.ClientConnectionConfiguration
				fldPath          *field.Path
			)

			BeforeEach(func() {
				gardenletconfigv1alpha1.SetObjectDefaults_GardenletConfiguration(cfg)
			})

			commonTests := func() {
				It("should allow default client connection configuration", func() {
					Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(BeEmpty())
				})

				It("should return errors because some values are invalid", func() {
					clientConnection.Burst = -1

					Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal(fldPath.Child("burst").String()),
						})),
					))
				})
			}

			Context("garden client connection", func() {
				BeforeEach(func() {
					clientConnection = &cfg.GardenClientConnection.ClientConnectionConfiguration
					fldPath = field.NewPath("gardenClientConnection")
				})

				commonTests()

				Context("kubeconfig validity", func() {
					It("should allow when config is not set", func() {
						Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(BeEmpty())
					})

					It("should allow valid configurations", func() {
						cfg.GardenClientConnection = &gardenletconfigv1alpha1.GardenClientConnection{
							KubeconfigValidity: &gardenletconfigv1alpha1.KubeconfigValidity{
								Validity:                        &metav1.Duration{Duration: time.Hour},
								AutoRotationJitterPercentageMin: ptr.To[int32](13),
								AutoRotationJitterPercentageMax: ptr.To[int32](37),
							},
						}

						Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(BeEmpty())
					})

					It("should forbid validity less than 10m", func() {
						cfg.GardenClientConnection = &gardenletconfigv1alpha1.GardenClientConnection{
							KubeconfigValidity: &gardenletconfigv1alpha1.KubeconfigValidity{
								Validity: &metav1.Duration{Duration: time.Second},
							},
						}

						Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("gardenClientConnection.kubeconfigValidity.validity"),
							"Detail": ContainSubstring("must be at least 10m"),
						}))))
					})

					It("should forbid auto rotation jitter percentage min less than 1", func() {
						cfg.GardenClientConnection = &gardenletconfigv1alpha1.GardenClientConnection{
							KubeconfigValidity: &gardenletconfigv1alpha1.KubeconfigValidity{
								AutoRotationJitterPercentageMin: ptr.To[int32](0),
							},
						}

						Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("gardenClientConnection.kubeconfigValidity.autoRotationJitterPercentageMin"),
							"Detail": ContainSubstring("must be at least 1"),
						}))))
					})

					It("should forbid auto rotation jitter percentage max more than 100", func() {
						cfg.GardenClientConnection = &gardenletconfigv1alpha1.GardenClientConnection{
							KubeconfigValidity: &gardenletconfigv1alpha1.KubeconfigValidity{
								AutoRotationJitterPercentageMax: ptr.To[int32](101),
							},
						}

						Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("gardenClientConnection.kubeconfigValidity.autoRotationJitterPercentageMax"),
							"Detail": ContainSubstring("must be at most 100"),
						}))))
					})

					It("should forbid auto rotation jitter percentage min equal max", func() {
						cfg.GardenClientConnection = &gardenletconfigv1alpha1.GardenClientConnection{
							KubeconfigValidity: &gardenletconfigv1alpha1.KubeconfigValidity{
								AutoRotationJitterPercentageMin: ptr.To[int32](13),
								AutoRotationJitterPercentageMax: ptr.To[int32](13),
							},
						}

						Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("gardenClientConnection.kubeconfigValidity.autoRotationJitterPercentageMin"),
							"Detail": ContainSubstring("minimum percentage must be less than maximum percentage"),
						}))))
					})

					It("should forbid auto rotation jitter percentage min higher than max", func() {
						cfg.GardenClientConnection = &gardenletconfigv1alpha1.GardenClientConnection{
							KubeconfigValidity: &gardenletconfigv1alpha1.KubeconfigValidity{
								AutoRotationJitterPercentageMin: ptr.To[int32](14),
								AutoRotationJitterPercentageMax: ptr.To[int32](13),
							},
						}

						Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("gardenClientConnection.kubeconfigValidity.autoRotationJitterPercentageMin"),
							"Detail": ContainSubstring("minimum percentage must be less than maximum percentage"),
						}))))
					})
				})
			})

			Context("seed client connection", func() {
				BeforeEach(func() {
					clientConnection = &cfg.SeedClientConnection.ClientConnectionConfiguration
					fldPath = field.NewPath("seedClientConnection")
				})

				commonTests()
			})

			Context("shoot client connection", func() {
				BeforeEach(func() {
					clientConnection = &cfg.ShootClientConnection.ClientConnectionConfiguration
					fldPath = field.NewPath("shootClientConnection")
				})

				commonTests()
			})
		})

		Context("leader election configuration", func() {
			BeforeEach(func() {
				gardenletconfigv1alpha1.SetObjectDefaults_GardenletConfiguration(cfg)
			})

			It("should allow not enabling leader election", func() {
				cfg.LeaderElection.LeaderElect = nil

				Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(BeEmpty())
			})

			It("should allow disabling leader election", func() {
				cfg.LeaderElection.LeaderElect = ptr.To(false)

				Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(BeEmpty())
			})

			It("should allow default leader election configuration with required fields", func() {
				Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(BeEmpty())
			})

			It("should reject leader election config with missing required fields", func() {
				cfg.LeaderElection.ResourceNamespace = ""

				Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("leaderElection.resourceNamespace"),
					})),
				))
			})
		})

		Context("shoot controller", func() {
			It("should forbid invalid configuration", func() {
				invalidConcurrentSyncs := -1

				cfg.Controllers.Shoot.ConcurrentSyncs = &invalidConcurrentSyncs
				cfg.Controllers.Shoot.ProgressReportPeriod = &metav1.Duration{Duration: -1}
				cfg.Controllers.Shoot.SyncPeriod = &metav1.Duration{Duration: -1}
				cfg.Controllers.Shoot.RetryDuration = &metav1.Duration{Duration: -1}

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.shoot.concurrentSyncs"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.shoot.progressReporterPeriod"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.shoot.syncPeriod"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.shoot.retryDuration"),
					})),
				))
			})

			It("should forbid too low values for the DNS TTL", func() {
				cfg.Controllers.Shoot.DNSEntryTTLSeconds = ptr.To(int64(-1))

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("controllers.shoot.dnsEntryTTLSeconds"),
				}))))
			})

			It("should forbid too high values for the DNS TTL", func() {
				cfg.Controllers.Shoot.DNSEntryTTLSeconds = ptr.To[int64](601)

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("controllers.shoot.dnsEntryTTLSeconds"),
				}))))
			})
		})

		Context("shootCare controller", func() {
			It("should forbid invalid configuration", func() {
				invalidConcurrentSyncs := -1

				cfg.Controllers.ShootCare.ConcurrentSyncs = &invalidConcurrentSyncs
				cfg.Controllers.ShootCare.SyncPeriod = &metav1.Duration{Duration: -1}
				cfg.Controllers.ShootCare.StaleExtensionHealthChecks = &gardenletconfigv1alpha1.StaleExtensionHealthChecks{Threshold: &metav1.Duration{Duration: -1}}
				cfg.Controllers.ShootCare.ManagedResourceProgressingThreshold = &metav1.Duration{Duration: -1}
				cfg.Controllers.ShootCare.ConditionThresholds = []gardenletconfigv1alpha1.ConditionThreshold{{Duration: metav1.Duration{Duration: -1}}}

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.shootCare.concurrentSyncs"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.shootCare.syncPeriod"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.shootCare.staleExtensionHealthChecks.threshold"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.shootCare.managedResourceProgressingThreshold"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.shootCare.conditionThresholds[0].duration"),
					})),
				))
			})
		})

		Context("managed seed controller", func() {
			It("should forbid invalid configuration", func() {
				invalidConcurrentSyncs := -1

				cfg.Controllers.ManagedSeed.ConcurrentSyncs = &invalidConcurrentSyncs
				cfg.Controllers.ManagedSeed.SyncPeriod = &metav1.Duration{Duration: -1}
				cfg.Controllers.ManagedSeed.WaitSyncPeriod = &metav1.Duration{Duration: -1}
				cfg.Controllers.ManagedSeed.SyncJitterPeriod = &metav1.Duration{Duration: -1}

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.managedSeed.concurrentSyncs"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.managedSeed.syncPeriod"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.managedSeed.waitSyncPeriod"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.managedSeed.syncJitterPeriod"),
					})),
				))
			})
		})

		Context("backup entry controller", func() {
			It("should forbid specifying purposes when not specifying hours", func() {
				cfg.Controllers.BackupEntry.DeletionGracePeriodHours = nil

				Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("controllers.backupEntry.deletionGracePeriodShootPurposes"),
					})),
				))
			})

			It("should allow valid purposes", func() {
				cfg.Controllers.BackupEntry.DeletionGracePeriodShootPurposes = []gardencorev1beta1.ShootPurpose{
					gardencorev1beta1.ShootPurposeEvaluation,
					gardencorev1beta1.ShootPurposeTesting,
					gardencorev1beta1.ShootPurposeDevelopment,
					gardencorev1beta1.ShootPurposeInfrastructure,
					gardencorev1beta1.ShootPurposeProduction,
				}

				Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(BeEmpty())
			})

			It("should forbid invalid purposes", func() {
				cfg.Controllers.BackupEntry.DeletionGracePeriodShootPurposes = []gardencorev1beta1.ShootPurpose{"does-not-exist"}

				Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("controllers.backupEntry.deletionGracePeriodShootPurposes[0]"),
					})),
				))
			})
		})

		Context("bastion controller", func() {
			It("should forbid invalid configuration", func() {
				invalidConcurrentSyncs := -1
				cfg.Controllers.Bastion.ConcurrentSyncs = &invalidConcurrentSyncs

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.bastion.concurrentSyncs"),
					})),
				))
			})
		})

		Context("network policy controller", func() {
			BeforeEach(func() {
				cfg.Controllers.NetworkPolicy = &gardenletconfigv1alpha1.NetworkPolicyControllerConfiguration{}
			})

			It("should return errors because concurrent syncs are < 0", func() {
				cfg.Controllers.NetworkPolicy.ConcurrentSyncs = ptr.To(-1)

				Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.networkPolicy.concurrentSyncs"),
					})),
				))
			})

			It("should return errors because some label selector is invalid", func() {
				cfg.Controllers.NetworkPolicy.AdditionalNamespaceSelectors = append(cfg.Controllers.NetworkPolicy.AdditionalNamespaceSelectors,
					metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
					metav1.LabelSelector{MatchLabels: map[string]string{"foo": "no/slash/allowed"}},
				)

				Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("controllers.networkPolicy.additionalNamespaceSelectors[1].matchLabels"),
					})),
				))
			})
		})

		Context("seed config", func() {
			It("should require a seedConfig", func() {
				cfg.SeedConfig = nil

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("seedConfig"),
				}))))
			})
		})

		Context("seed template", func() {
			It("should forbid invalid fields in seed template", func() {
				cfg.SeedConfig.Spec.Networks.Nodes = ptr.To("")

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("seedConfig.spec.networks.nodes"),
					})),
				))
			})
		})

		Context("resources", func() {
			It("should forbid reserved greater than capacity", func() {
				cfg.Resources = &gardenletconfigv1alpha1.ResourcesConfiguration{
					Capacity: corev1.ResourceList{
						"foo": resource.MustParse("42"),
					},
					Reserved: corev1.ResourceList{
						"foo": resource.MustParse("43"),
					},
				}

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("resources.reserved.foo"),
				}))))
			})

			It("should forbid reserved without capacity", func() {
				cfg.Resources = &gardenletconfigv1alpha1.ResourcesConfiguration{
					Reserved: corev1.ResourceList{
						"foo": resource.MustParse("42"),
					},
				}

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("resources.reserved.foo"),
				}))))
			})
		})

		Context("sni", func() {
			BeforeEach(func() {
				cfg.SNI = &gardenletconfigv1alpha1.SNI{Ingress: &gardenletconfigv1alpha1.SNIIngress{}}
			})

			It("should pass as sni config contains a valid external service ip", func() {
				cfg.SNI.Ingress.ServiceExternalIP = ptr.To("1.1.1.1")

				errorList := ValidateGardenletConfiguration(cfg, nil, false)
				Expect(errorList).To(BeEmpty())
			})

			It("should forbid as sni config contains an empty external service ip", func() {
				cfg.SNI.Ingress.ServiceExternalIP = ptr.To("")

				errorList := ValidateGardenletConfiguration(cfg, nil, false)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("sni.ingress.serviceExternalIP"),
				}))))
			})

			It("should forbid as sni config contains an invalid external service ip", func() {
				cfg.SNI.Ingress.ServiceExternalIP = ptr.To("a.b.c.d")

				errorList := ValidateGardenletConfiguration(cfg, nil, false)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("sni.ingress.serviceExternalIP"),
				}))))
			})
		})

		Context("exposureClassHandlers", func() {
			BeforeEach(func() {
				cfg.ExposureClassHandlers = []gardenletconfigv1alpha1.ExposureClassHandler{
					{
						Name: "test",
						LoadBalancerService: gardenletconfigv1alpha1.LoadBalancerServiceConfig{
							Annotations: map[string]string{"test": "foo"},
						},
						SNI: &gardenletconfigv1alpha1.SNI{Ingress: &gardenletconfigv1alpha1.SNIIngress{}},
					},
				}
			})

			It("should pass valid exposureClassHandler", func() {
				errorList := ValidateGardenletConfiguration(cfg, nil, false)
				Expect(errorList).To(BeEmpty())
			})

			It("should fail as exposureClassHandler name is no DNS1123 label with zero length", func() {
				cfg.ExposureClassHandlers[0].Name = ""

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("exposureClassHandlers[0].name"),
				}))))
			})

			It("should fail as exposureClassHandler name is no DNS1123 label", func() {
				cfg.ExposureClassHandlers[0].Name = "TE:ST"

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("exposureClassHandlers[0].name"),
				}))))
			})

			Context("serviceExternalIP", func() {
				It("should allow to use an external service ip as loadbalancer ip is valid", func() {
					cfg.ExposureClassHandlers[0].SNI.Ingress.ServiceExternalIP = ptr.To("1.1.1.1")

					errorList := ValidateGardenletConfiguration(cfg, nil, false)

					Expect(errorList).To(BeEmpty())
				})

				It("should allow to use an external service ip", func() {
					cfg.ExposureClassHandlers[0].SNI.Ingress.ServiceExternalIP = ptr.To("1.1.1.1")

					errorList := ValidateGardenletConfiguration(cfg, nil, false)

					Expect(errorList).To(BeEmpty())
				})

				It("should forbid to use an empty external service ip", func() {
					cfg.ExposureClassHandlers[0].SNI.Ingress.ServiceExternalIP = ptr.To("")

					errorList := ValidateGardenletConfiguration(cfg, nil, false)
					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("exposureClassHandlers[0].sni.ingress.serviceExternalIP"),
					}))))
				})

				It("should forbid to use an invalid external service ip", func() {
					cfg.ExposureClassHandlers[0].SNI.Ingress.ServiceExternalIP = ptr.To("a.b.c.d")

					errorList := ValidateGardenletConfiguration(cfg, nil, false)
					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("exposureClassHandlers[0].sni.ingress.serviceExternalIP"),
					}))))
				})
			})
		})

		Context("nodeToleration", func() {
			It("should pass with unset toleration options", func() {
				cfg.NodeToleration = nil

				Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(BeEmpty())
			})

			It("should pass with unset toleration seconds", func() {
				cfg.NodeToleration = &gardenletconfigv1alpha1.NodeToleration{
					DefaultNotReadyTolerationSeconds:    nil,
					DefaultUnreachableTolerationSeconds: nil,
				}

				Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(BeEmpty())
			})

			It("should pass with valid toleration options", func() {
				cfg.NodeToleration = &gardenletconfigv1alpha1.NodeToleration{
					DefaultNotReadyTolerationSeconds:    ptr.To[int64](60),
					DefaultUnreachableTolerationSeconds: ptr.To[int64](120),
				}

				Expect(ValidateGardenletConfiguration(cfg, nil, false)).To(BeEmpty())
			})

			It("should fail with invalid toleration options", func() {
				cfg.NodeToleration = &gardenletconfigv1alpha1.NodeToleration{
					DefaultNotReadyTolerationSeconds:    ptr.To(int64(-1)),
					DefaultUnreachableTolerationSeconds: ptr.To(int64(-2)),
				}

				errorList := ValidateGardenletConfiguration(cfg, nil, false)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("nodeToleration.defaultNotReadyTolerationSeconds"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("nodeToleration.defaultUnreachableTolerationSeconds"),
					}))),
				)
			})
		})
	})

	Describe("#ValidateGardenletConfigurationUpdate", func() {
		It("should allow valid configuration updates", func() {
			errorList := ValidateGardenletConfigurationUpdate(cfg, cfg, nil)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid changes to immutable fields in seed template", func() {
			newCfg := cfg.DeepCopy()
			newCfg.SeedConfig.Spec.Networks.Pods = "100.97.0.0/11"

			errorList := ValidateGardenletConfigurationUpdate(newCfg, cfg, nil)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("seedConfig.spec.networks.pods"),
					"Detail": Equal("field is immutable"),
				})),
			))
		})
	})
})
