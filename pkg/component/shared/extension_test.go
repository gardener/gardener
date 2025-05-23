// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared_test

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	extensionpkg "github.com/gardener/gardener/pkg/component/extensions/extension"
	. "github.com/gardener/gardener/pkg/component/shared"
)

var _ = Describe("Extension", func() {
	Describe("#NewExtension", func() {
		var (
			ctrl *gomock.Controller

			ctx          context.Context
			log          logr.Logger
			gardenClient client.Client
			seedClient   client.Client

			lifecycle      *gardencorev1beta1.ControllerResourceLifecycle
			extensionKind  = extensionsv1alpha1.ExtensionResource
			providerConfig = runtime.RawExtension{
				Raw: []byte("key: value"),
			}

			fooExtensionType         string
			fooReconciliationTimeout metav1.Duration
			fooRegistration          gardencorev1beta1.ControllerRegistration
			fooExtension             gardencorev1beta1.Extension

			barExtensionType                      string
			barRegistration                       gardencorev1beta1.ControllerRegistration
			barRegistrationSupportedForWorkerless gardencorev1beta1.ControllerRegistration
			barExtension                          gardencorev1beta1.Extension
			barExtensionDisabled                  gardencorev1beta1.Extension
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())

			ctx = context.Background()
			log = logr.Discard()
			gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
			seedClient = fakeclient.NewClientBuilder().Build()

			extensionKind = extensionsv1alpha1.ExtensionResource
			providerConfig = runtime.RawExtension{
				Raw: []byte("key: value"),
			}

			fooExtensionType = "foo"
			fooReconciliationTimeout = metav1.Duration{Duration: 5 * time.Minute}
			fooRegistration = gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{
							Kind:             extensionKind,
							Type:             fooExtensionType,
							ReconcileTimeout: &fooReconciliationTimeout,
						},
					},
				},
			}
			fooExtension = gardencorev1beta1.Extension{
				Type:           fooExtensionType,
				ProviderConfig: &providerConfig,
			}

			barExtensionType = "bar"
			barRegistration = gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bar",
				},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{
							Kind:       extensionKind,
							Type:       barExtensionType,
							AutoEnable: []gardencorev1beta1.ClusterType{"shoot"},
						},
					},
				},
			}
			barRegistrationSupportedForWorkerless = gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bar-wl",
				},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{
							Kind:                extensionKind,
							Type:                barExtensionType,
							AutoEnable:          []gardencorev1beta1.ClusterType{"shoot"},
							WorkerlessSupported: ptr.To(true),
						},
					},
				},
			}
			barExtension = gardencorev1beta1.Extension{
				Type:           barExtensionType,
				ProviderConfig: &providerConfig,
			}
			barExtensionDisabled = gardencorev1beta1.Extension{
				Type:           barExtensionType,
				ProviderConfig: &providerConfig,
				Disabled:       ptr.To(true),
			}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		test := func(registrations []gardencorev1beta1.ControllerRegistration, extensions []gardencorev1beta1.Extension, class extensionsv1alpha1.ExtensionClass, workerless bool, conditionMatcher gomegatypes.GomegaMatcher) {
			GinkgoHelper()

			for _, registration := range registrations {
				Expect(gardenClient.Create(ctx, &registration)).To(Succeed())
			}

			ext, err := NewExtension(ctx, log, gardenClient, seedClient, "test", class, extensions, workerless)
			Expect(err).NotTo(HaveOccurred())

			extensionObjs := ext.Extensions()
			for _, extensionObj := range extensionObjs {
				Expect(extensionObj.GetNamespace()).To(Equal("test"), fmt.Sprintf("expected to have namespace %q, but got %q", "test", extensionObj.GetNamespace()))
			}

			Expect(extensionObjs).To(conditionMatcher)
		}

		It("should return no extensions when no extensions are configured and registered", func() {
			test(nil, nil, extensionsv1alpha1.ExtensionClassShoot, false, BeEmpty())
		})

		It("should return no extensions when extension is not registered", func() {
			test(nil, []gardencorev1beta1.Extension{{Type: fooExtensionType}}, extensionsv1alpha1.ExtensionClassShoot, false, BeEmpty())
		})

		It("should return no extension when no extension is configured", func() {
			test([]gardencorev1beta1.ControllerRegistration{fooRegistration}, nil, extensionsv1alpha1.ExtensionClassShoot, false, BeEmpty())
		})

		It("should return the configured expected extension", func() {
			test([]gardencorev1beta1.ControllerRegistration{fooRegistration},
				[]gardencorev1beta1.Extension{fooExtension},
				extensionsv1alpha1.ExtensionClassShoot,
				false,
				HaveKeyWithValue(
					Equal(fooExtensionType),
					MatchAllFields(
						Fields{
							"Extension": MatchFields(IgnoreExtras, Fields{
								"Spec": MatchFields(IgnoreExtras, Fields{
									"DefaultSpec": MatchAllFields(Fields{
										"Type":           Equal(fooExtensionType),
										"ProviderConfig": PointTo(Equal(providerConfig)),
										"Class":          BeNil(),
									}),
								}),
							}),
							"Timeout":   Equal(fooReconciliationTimeout.Duration),
							"Lifecycle": Equal(lifecycle),
						},
					),
				),
			)
		})

		It("should return the expected extension when multiple are registered", func() {
			test(
				[]gardencorev1beta1.ControllerRegistration{
					fooRegistration,
					barRegistration,
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "another-extension",
						},
						Spec: gardencorev1beta1.ControllerRegistrationSpec{
							Resources: []gardencorev1beta1.ControllerResource{
								{
									Kind: "kind",
									Type: "type",
								},
							},
						},
					},
				},
				[]gardencorev1beta1.Extension{barExtension},
				extensionsv1alpha1.ExtensionClassShoot,
				false,
				HaveKeyWithValue(
					Equal(barExtensionType),
					MatchAllFields(
						Fields{
							"Extension": MatchFields(IgnoreExtras, Fields{
								"Spec": MatchAllFields(Fields{
									"DefaultSpec": MatchAllFields(Fields{
										"Type":           Equal(barExtensionType),
										"ProviderConfig": PointTo(Equal(providerConfig)),
										"Class":          BeNil(),
									}),
								}),
							}),
							"Timeout":   Equal(extensionpkg.DefaultTimeout),
							"Lifecycle": Equal(lifecycle),
						},
					),
				),
			)
		})

		When("automatically enabled", func() {
			Context("for shoots", func() {
				BeforeEach(func() {
					fooRegistration.Spec.Resources[0].AutoEnable = []gardencorev1beta1.ClusterType{"shoot"}
				})

				It("should return the extension for class shoot", func() {
					test(
						[]gardencorev1beta1.ControllerRegistration{
							fooRegistration,
						},
						nil,
						extensionsv1alpha1.ExtensionClassShoot,
						false,
						HaveKeyWithValue(
							Equal(fooExtensionType),
							MatchAllFields(
								Fields{
									"Extension": MatchFields(IgnoreExtras, Fields{
										"Spec": MatchAllFields(Fields{
											"DefaultSpec": MatchAllFields(Fields{
												"Type":           Equal(fooExtensionType),
												"ProviderConfig": BeNil(),
												"Class":          BeNil(),
											}),
										}),
									}),
									"Timeout":   Equal(fooReconciliationTimeout.Duration),
									"Lifecycle": Equal(lifecycle),
								},
							),
						),
					)
				})

				It("should not return the extension for class seed", func() {
					test(
						[]gardencorev1beta1.ControllerRegistration{
							fooRegistration,
						},
						nil,
						extensionsv1alpha1.ExtensionClassSeed,
						false,
						BeEmpty(),
					)
				})
			})

			Context("for seeds", func() {
				BeforeEach(func() {
					fooRegistration.Spec.Resources[0].AutoEnable = []gardencorev1beta1.ClusterType{"seed"}
				})

				It("should return the extension for class seed", func() {
					test(
						[]gardencorev1beta1.ControllerRegistration{
							fooRegistration,
						},
						nil,
						extensionsv1alpha1.ExtensionClassSeed,
						false,
						HaveKeyWithValue(
							Equal(fooExtensionType),
							MatchAllFields(
								Fields{
									"Extension": MatchFields(IgnoreExtras, Fields{
										"Spec": MatchAllFields(Fields{
											"DefaultSpec": MatchAllFields(Fields{
												"Type":           Equal(fooExtensionType),
												"ProviderConfig": BeNil(),
												"Class":          BeNil(),
											}),
										}),
									}),
									"Timeout":   Equal(fooReconciliationTimeout.Duration),
									"Lifecycle": Equal(lifecycle),
								},
							),
						),
					)
				})

				It("should not return the extension for class shoot", func() {
					test(
						[]gardencorev1beta1.ControllerRegistration{
							fooRegistration,
						},
						nil,
						extensionsv1alpha1.ExtensionClassShoot,
						false,
						BeEmpty(),
					)
				})
			})

			Context("for all clusters", func() {
				BeforeEach(func() {
					fooRegistration.Spec.Resources[0].AutoEnable = []gardencorev1beta1.ClusterType{"shoot", "seed"}
				})

				It("should return the extension for class shoot", func() {
					test(
						[]gardencorev1beta1.ControllerRegistration{
							fooRegistration,
						},
						nil,
						extensionsv1alpha1.ExtensionClassShoot,
						false,
						HaveKeyWithValue(
							Equal(fooExtensionType),
							MatchAllFields(
								Fields{
									"Extension": MatchFields(IgnoreExtras, Fields{
										"Spec": MatchAllFields(Fields{
											"DefaultSpec": MatchAllFields(Fields{
												"Type":           Equal(fooExtensionType),
												"ProviderConfig": BeNil(),
												"Class":          BeNil(),
											}),
										}),
									}),
									"Timeout":   Equal(fooReconciliationTimeout.Duration),
									"Lifecycle": Equal(lifecycle),
								},
							),
						),
					)
				})

				It("should return the extension for class seed", func() {
					test(
						[]gardencorev1beta1.ControllerRegistration{
							fooRegistration,
						},
						nil,
						extensionsv1alpha1.ExtensionClassSeed,
						false,
						HaveKeyWithValue(
							Equal(fooExtensionType),
							MatchAllFields(
								Fields{
									"Extension": MatchFields(IgnoreExtras, Fields{
										"Spec": MatchAllFields(Fields{
											"DefaultSpec": MatchAllFields(Fields{
												"Type":           Equal(fooExtensionType),
												"ProviderConfig": BeNil(),
												"Class":          BeNil(),
											}),
										}),
									}),
									"Timeout":   Equal(fooReconciliationTimeout.Duration),
									"Lifecycle": Equal(lifecycle),
								},
							),
						),
					)
				})
			})

			Context("none", func() {
				BeforeEach(func() {
					fooRegistration.Spec.Resources[0].AutoEnable = nil
				})

				It("should return the extension for class shoot", func() {
					test(
						[]gardencorev1beta1.ControllerRegistration{
							fooRegistration,
						},
						nil,
						extensionsv1alpha1.ExtensionClassShoot,
						false,
						BeEmpty(),
					)
				})

				It("should return the extension for class seed", func() {
					test(
						[]gardencorev1beta1.ControllerRegistration{
							fooRegistration,
						},
						nil,
						extensionsv1alpha1.ExtensionClassSeed,
						false,
						BeEmpty(),
					)
				})
			})

			It("should return the expected extension", func() {
				test([]gardencorev1beta1.ControllerRegistration{barRegistration},
					nil,
					extensionsv1alpha1.ExtensionClassShoot,
					false,
					HaveKeyWithValue(
						Equal(barExtensionType),
						MatchAllFields(
							Fields{
								"Extension": MatchFields(IgnoreExtras, Fields{
									"Spec": MatchAllFields(Fields{
										"DefaultSpec": MatchAllFields(Fields{
											"Type":           Equal(barExtensionType),
											"ProviderConfig": BeNil(),
											"Class":          BeNil(),
										}),
									}),
								}),
								"Timeout":   Equal(extensionpkg.DefaultTimeout),
								"Lifecycle": Equal(lifecycle),
							},
						),
					),
				)
			})

			It("should return no extension when explicitly disabled", func() {
				test([]gardencorev1beta1.ControllerRegistration{barRegistration}, []gardencorev1beta1.Extension{barExtensionDisabled}, extensionsv1alpha1.ExtensionClassShoot, false, BeEmpty())
			})

			It("should return the expected extension", func() {
				test(
					[]gardencorev1beta1.ControllerRegistration{fooRegistration, barRegistration},
					[]gardencorev1beta1.Extension{fooExtension, barExtensionDisabled},
					extensionsv1alpha1.ExtensionClassShoot,
					false,
					SatisfyAll(
						HaveLen(1),
						HaveKeyWithValue(
							Equal(fooExtensionType),
							MatchAllFields(
								Fields{
									"Extension": MatchFields(IgnoreExtras, Fields{
										"Spec": MatchFields(IgnoreExtras, Fields{
											"DefaultSpec": MatchAllFields(Fields{
												"Type":           Equal(fooExtensionType),
												"ProviderConfig": PointTo(Equal(providerConfig)),
												"Class":          BeNil(),
											}),
										}),
									}),
									"Timeout":   Equal(fooReconciliationTimeout.Duration),
									"Lifecycle": Equal(lifecycle),
								},
							),
						),
					),
				)
			})
		})

		Context("Workerless", func() {
			It("should return automatically enabled for workerless", func() {
				test(
					[]gardencorev1beta1.ControllerRegistration{
						barRegistrationSupportedForWorkerless,
					},
					[]gardencorev1beta1.Extension{},
					extensionsv1alpha1.ExtensionClassShoot,
					true,
					HaveKeyWithValue(
						Equal(barExtensionType),
						MatchFields(IgnoreExtras, Fields{
							"Extension": MatchFields(IgnoreExtras, Fields{
								"Spec": MatchFields(IgnoreExtras, Fields{
									"DefaultSpec": MatchFields(IgnoreExtras, Fields{
										"Type":  Equal(barExtensionType),
										"Class": BeNil(),
									}),
								}),
							}),
						},
						),
					),
				)
			})

			It("should return no extension when automatically enabled but not supported for workerless", func() {
				test(
					[]gardencorev1beta1.ControllerRegistration{
						barRegistration,
					},
					[]gardencorev1beta1.Extension{},
					extensionsv1alpha1.ExtensionClassShoot,
					true,
					BeEmpty())
			})
		})
	})
})
