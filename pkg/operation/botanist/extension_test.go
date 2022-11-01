// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist_test

import (
	"context"
	"errors"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	extensionpkg "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/extension"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type errorClient struct {
	client.Client
	err error
}

func (e *errorClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return e.err
}

func (e *errorClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return e.err
}

func (e *errorClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return e.err
}

var _ = Describe("Extensions", func() {

	const (
		namespaceName = "shoot--name--space"
	)
	var (
		gardenFakeClient client.Client
		seedFakeClient   client.Client
		errClient        client.Client
		errClientSet     *fakekubernetes.ClientSet
		botanist         *Botanist
		ctx              = context.TODO()
		shootState       = &gardencorev1alpha1.ShootState{}
		log              logr.Logger

		fakeError     error
		extensionKind = extensionsv1alpha1.ExtensionResource
	)

	BeforeEach(func() {
		gardenFakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		seedFakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeSeedClientSet := fakekubernetes.NewClientSetBuilder().WithClient(seedFakeClient).Build()

		fakeError = errors.New("fake-err")
		errClient = &errorClient{err: fakeError}
		errClientSet = fakekubernetes.NewClientSetBuilder().WithClient(errClient).Build()

		logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
		log = logf.Log.WithName("extensions")

		botanist = &Botanist{Operation: &operation.Operation{
			GardenClient:  gardenFakeClient,
			SeedClientSet: fakeSeedClientSet,
			Logger:        log,
			Shoot: &shootpkg.Shoot{

				SeedNamespace: namespaceName,
			},
		}}
		botanist.SetShootState(shootState)
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
	})

	Describe("#DefaultExtension", func() {
		var (
			providerConfig = runtime.RawExtension{
				Raw: []byte("key: value"),
			}

			foo                      = "foo"
			fooReconciliationTimeout = metav1.Duration{Duration: 5 * time.Minute}
			fooRegistration          = gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: foo},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{
							Kind:             extensionKind,
							Type:             foo,
							ReconcileTimeout: &fooReconciliationTimeout,
						},
					},
				},
			}
			fooExtension = gardencorev1beta1.Extension{
				Type:           foo,
				ProviderConfig: &providerConfig,
			}

			bar             = "bar"
			barRegistration = gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: bar},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{
							Kind:            extensionKind,
							Type:            bar,
							GloballyEnabled: pointer.Bool(true),
						},
					},
				},
			}
			barExtension = gardencorev1beta1.Extension{
				Type:           bar,
				ProviderConfig: &providerConfig,
			}
			barExtensionDisabled = gardencorev1beta1.Extension{
				Type:           bar,
				ProviderConfig: &providerConfig,
				Disabled:       pointer.Bool(true),
			}

			emptyLifecycle *gardencorev1beta1.ControllerResourceLifecycle
		)

		It("should return the error because listing failed", func() {
			botanist.GardenClient = errClient
			ext, err := botanist.DefaultExtension(ctx)
			Expect(ext).To(BeNil())
			Expect(err).To(MatchError(fakeError))
		})

		DescribeTable("#DefaultExtension",
			func(registrations []gardencorev1beta1.ControllerRegistration, extensions []gardencorev1beta1.Extension, conditionMatcher gomegatypes.GomegaMatcher) {
				botanist.Shoot.GetInfo().Spec.Extensions = extensions
				for _, registration := range registrations {
					Expect(gardenFakeClient.Create(ctx, &registration)).To(Succeed())
				}

				ext, err := botanist.DefaultExtension(ctx)
				Expect(err).To(BeNil())
				Expect(ext.Extensions()).To(conditionMatcher)
			},

			Entry(
				"No extensions",
				nil,
				nil,
				BeEmpty(),
			),
			Entry(
				"Extension w/o registration",
				nil,
				[]gardencorev1beta1.Extension{{Type: foo}},
				BeEmpty(),
			),
			Entry(
				"Extensions w/ registration",
				[]gardencorev1beta1.ControllerRegistration{fooRegistration},
				[]gardencorev1beta1.Extension{fooExtension},
				HaveKeyWithValue(
					Equal(foo),
					MatchAllFields(
						Fields{
							"Extension": MatchFields(IgnoreExtras, Fields{
								"Spec": MatchFields(IgnoreExtras, Fields{
									"DefaultSpec": MatchAllFields(Fields{
										"Type":           Equal(foo),
										"ProviderConfig": PointTo(Equal(providerConfig)),
									}),
								}),
							}),
							"Timeout":   Equal(fooReconciliationTimeout.Duration),
							"Lifecycle": Equal(emptyLifecycle),
						},
					),
				),
			),
			Entry(
				"Registration w/o extension",
				[]gardencorev1beta1.ControllerRegistration{fooRegistration},
				nil,
				BeEmpty(),
			),
			Entry(
				"Globally enabled extension registration, w/o extension",
				[]gardencorev1beta1.ControllerRegistration{barRegistration},
				nil,
				HaveKeyWithValue(
					Equal(bar),
					MatchAllFields(
						Fields{
							"Extension": MatchFields(IgnoreExtras, Fields{
								"Spec": MatchAllFields(Fields{
									"DefaultSpec": MatchAllFields(Fields{
										"Type":           Equal(bar),
										"ProviderConfig": BeNil(),
									}),
								}),
							}),
							"Timeout":   Equal(extensionpkg.DefaultTimeout),
							"Lifecycle": Equal(emptyLifecycle),
						},
					),
				),
			),
			Entry(
				"Globally enabled extension registration but explicitly disabled",
				[]gardencorev1beta1.ControllerRegistration{barRegistration},
				[]gardencorev1beta1.Extension{barExtensionDisabled},
				BeEmpty(),
			),
			Entry(
				"Multiple registration but a globally one is explicitly disabled",
				[]gardencorev1beta1.ControllerRegistration{fooRegistration, barRegistration},
				[]gardencorev1beta1.Extension{fooExtension, barExtensionDisabled},
				SatisfyAll(
					HaveLen(1),
					HaveKeyWithValue(
						Equal(foo),
						MatchAllFields(
							Fields{
								"Extension": MatchFields(IgnoreExtras, Fields{
									"Spec": MatchFields(IgnoreExtras, Fields{
										"DefaultSpec": MatchAllFields(Fields{
											"Type":           Equal(foo),
											"ProviderConfig": PointTo(Equal(providerConfig)),
										}),
									}),
								}),
								"Timeout":   Equal(fooReconciliationTimeout.Duration),
								"Lifecycle": Equal(emptyLifecycle),
							},
						),
					),
				),
			),
			Entry(
				"Multiple registrations, w/ one extension",
				[]gardencorev1beta1.ControllerRegistration{
					fooRegistration,
					barRegistration,
					{
						ObjectMeta: metav1.ObjectMeta{Name: "kind"},
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
				HaveKeyWithValue(
					Equal(bar),
					MatchAllFields(
						Fields{
							"Extension": MatchFields(IgnoreExtras, Fields{
								"Spec": MatchAllFields(Fields{
									"DefaultSpec": MatchAllFields(Fields{
										"Type":           Equal(bar),
										"ProviderConfig": PointTo(Equal(providerConfig)),
									}),
								}),
							}),
							"Timeout":   Equal(extensionpkg.DefaultTimeout),
							"Lifecycle": Equal(emptyLifecycle),
						},
					),
				),
			),
		)
	})

	Describe("#DeployExtensions", func() {
		var (
			foo                      = "foo"
			fooReconciliationTimeout = metav1.Duration{Duration: 5 * time.Minute}
			fooRegistration          gardencorev1beta1.ControllerRegistration
		)
		BeforeEach(func() {
			fooRegistration = gardencorev1beta1.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{Name: "foo"},
				Spec: gardencorev1beta1.ControllerRegistrationSpec{
					Resources: []gardencorev1beta1.ControllerResource{
						{
							Kind:             extensionKind,
							Type:             foo,
							ReconcileTimeout: &fooReconciliationTimeout,
							GloballyEnabled:  pointer.Bool(true),
						},
					},
				},
			}
			Expect(gardenFakeClient.Create(ctx, &fooRegistration)).To(Succeed())
			extension, err := botanist.DefaultExtension(ctx)
			Expect(err).NotTo(HaveOccurred())
			botanist.Shoot.Components = &shootpkg.Components{
				Extensions: &shootpkg.Extensions{
					Extension: extension,
				},
			}
		})

		Context("deploy", func() {
			It("should deploy successfully", func() {
				Expect(botanist.DeployExtensionsAfterKubeAPIServer(ctx)).To(Succeed())
				ex := &extensionsv1alpha1.Extension{ObjectMeta: metav1.ObjectMeta{
					Name:      foo,
					Namespace: namespaceName,
				}}
				Expect(seedFakeClient.Get(ctx, client.ObjectKeyFromObject(ex), ex)).To(Succeed())
				Expect(ex.Annotations[gardencorev1beta1constants.GardenerOperation]).To(Equal(gardencorev1beta1constants.GardenerOperationReconcile))
			})

			It("should return the error during deployment", func() {
				botanist.SeedClientSet = errClientSet
				extension, err := botanist.DefaultExtension(ctx)
				Expect(err).NotTo(HaveOccurred())
				botanist.Shoot.Components = &shootpkg.Components{
					Extensions: &shootpkg.Extensions{
						Extension: extension,
					},
				}
				Expect(botanist.DeployExtensionsAfterKubeAPIServer(ctx)).To(MatchError(fakeError))
			})
		})

		Context("restore", func() {
			BeforeEach(func() {
				botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Status: gardencorev1beta1.ShootStatus{
						LastOperation: &gardencorev1beta1.LastOperation{
							Type: gardencorev1beta1.LastOperationTypeRestore,
						},
					},
				})
			})

			It("should restore successfully", func() {
				Expect(botanist.DeployExtensionsAfterKubeAPIServer(ctx)).To(Succeed())
				ex := &extensionsv1alpha1.Extension{ObjectMeta: metav1.ObjectMeta{
					Name:      foo,
					Namespace: namespaceName,
				}}
				Expect(seedFakeClient.Get(ctx, client.ObjectKeyFromObject(ex), ex)).To(Succeed())
				Expect(ex.Annotations[gardencorev1beta1constants.GardenerOperation]).To(Equal(gardencorev1beta1constants.GardenerOperationRestore))
			})

			It("should return the error during restoration", func() {
				botanist.SeedClientSet = errClientSet
				extension, err := botanist.DefaultExtension(ctx)
				Expect(err).NotTo(HaveOccurred())
				botanist.Shoot.Components = &shootpkg.Components{
					Extensions: &shootpkg.Extensions{
						Extension: extension,
					},
				}
				Expect(botanist.DeployExtensionsAfterKubeAPIServer(ctx)).To(MatchError(fakeError))
			})
		})
	})
})
