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
							Kind:            extensionKind,
							Type:            barExtensionType,
							GloballyEnabled: ptr.To(true),
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
							GloballyEnabled:     ptr.To(true),
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

		test := func(registrations []gardencorev1beta1.ControllerRegistration, extensions []gardencorev1beta1.Extension, workerless bool, conditionMatcher gomegatypes.GomegaMatcher) {
			GinkgoHelper()

			for _, registration := range registrations {
				Expect(gardenClient.Create(ctx, &registration)).To(Succeed())
			}

			ext, err := NewExtension(ctx, log, gardenClient, seedClient, "test", extensionsv1alpha1.ExtensionClassShoot, extensions, workerless)
			Expect(err).NotTo(HaveOccurred())

			extensionObjs := ext.Extensions()
			for _, extensionObj := range extensionObjs {
				Expect(extensionObj.GetNamespace()).To(Equal("test"), fmt.Sprintf("expected to have namespace %q, but got %q", "test", extensionObj.GetNamespace()))
			}

			Expect(extensionObjs).To(conditionMatcher)
		}

		It("should return no extensions when no extensions are configured and registered", func() {
			test(nil, nil, false, BeEmpty())
		})

		It("should return no extensions when extension is not registered", func() {
			test(nil, []gardencorev1beta1.Extension{{Type: fooExtensionType}}, false, BeEmpty())
		})

		It("should return no extension when no extension is configured", func() {
			test([]gardencorev1beta1.ControllerRegistration{fooRegistration}, nil, false, BeEmpty())
		})

		It("should return the configured expected extension", func() {
			test([]gardencorev1beta1.ControllerRegistration{fooRegistration},
				[]gardencorev1beta1.Extension{fooExtension},
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

		Context("Globally enabled", func() {
			It("should return the expected extension", func() {
				test([]gardencorev1beta1.ControllerRegistration{barRegistration},
					nil,
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
				test([]gardencorev1beta1.ControllerRegistration{barRegistration}, []gardencorev1beta1.Extension{barExtensionDisabled}, false, BeEmpty())
			})

			It("should return the expected extension", func() {
				test(
					[]gardencorev1beta1.ControllerRegistration{fooRegistration, barRegistration},
					[]gardencorev1beta1.Extension{fooExtension, barExtensionDisabled},
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
			It("should return globally enabled for workerless", func() {
				test(
					[]gardencorev1beta1.ControllerRegistration{
						barRegistrationSupportedForWorkerless,
					},
					[]gardencorev1beta1.Extension{},
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

			It("should return no extension when globally enabled but not supported for workerless", func() {
				test(
					[]gardencorev1beta1.ControllerRegistration{
						barRegistration,
					},
					[]gardencorev1beta1.Extension{},
					true,
					BeEmpty())
			})
		})
	})
})
