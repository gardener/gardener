// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package operation_test

import (
	"context"
	"errors"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mocktime "github.com/gardener/gardener/pkg/mock/go/time"
	. "github.com/gardener/gardener/pkg/operation"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("operation", func() {
	ctx := context.TODO()

	DescribeTable("#ComputeIngressHost", func(prefix, shootName, projectName, storedTechnicalID, domain string, matcher gomegatypes.GomegaMatcher) {
		var (
			seed = &gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Ingress: &gardencorev1beta1.Ingress{
						Domain: domain,
					},
				},
			}
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name: shootName,
				},
			}
			o = &Operation{
				Seed:  &seedpkg.Seed{},
				Shoot: &shootpkg.Shoot{},
			}
		)

		shoot.Status = gardencorev1beta1.ShootStatus{
			TechnicalID: storedTechnicalID,
		}
		shoot.Status.TechnicalID = shootpkg.ComputeTechnicalID(projectName, shoot)

		o.Seed.SetInfo(seed)
		o.Shoot.SetInfo(shoot)

		Expect(o.ComputeIngressHost(prefix)).To(matcher)
	},
		Entry("ingress calculation (no stored technical ID)",
			"t",
			"fooShoot",
			"barProject",
			"",
			"ingress.seed.example.com",
			Equal("t-barProject--fooShoot.ingress.seed.example.com"),
		),
		Entry("ingress calculation (historic stored technical ID with a single dash)",
			"t",
			"fooShoot",
			"barProject",
			"shoot-barProject--fooShoot",
			"ingress.seed.example.com",
			Equal("t-barProject--fooShoot.ingress.seed.example.com")),
		Entry("ingress calculation (current stored technical ID with two dashes)",
			"t",
			"fooShoot",
			"barProject",
			"shoot--barProject--fooShoot",
			"ingress.seed.example.com",
			Equal("t-barProject--fooShoot.ingress.seed.example.com")),
	)

	Context("ShootState", func() {
		var (
			shootState   *gardencorev1beta1.ShootState
			shoot        *gardencorev1beta1.Shoot
			ctrl         *gomock.Controller
			gardenClient *mockclient.MockClient
			o            *Operation
			gr           = schema.GroupResource{Resource: "ShootStates"}
			fakeErr      = errors.New("fake")
		)

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fakeShootName",
					Namespace: "fakeShootNS",
				},
			}
			shootState = &gardencorev1beta1.ShootState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shoot.Name,
					Namespace: shoot.Namespace,
				},
			}

			ctrl = gomock.NewController(GinkgoT())
			gardenClient = mockclient.NewMockClient(ctrl)
			o = &Operation{
				GardenClient: gardenClient,
				Shoot:        &shootpkg.Shoot{},
			}
			o.Shoot.SetInfo(shoot)
		})

		Describe("#EnsureShootStateExists", func() {

			It("should create ShootState and add it to the Operation object", func() {
				gomock.InOrder(
					gardenClient.EXPECT().Create(ctx, shootState).Return(nil),
					gardenClient.EXPECT().Get(ctx, kubernetesutils.Key("fakeShootNS", "fakeShootName"), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootState{})),
				)

				Expect(o.EnsureShootStateExists(ctx)).To(Succeed())

				Expect(o.GetShootState()).To(Equal(shootState))
			})

			It("should succeed and update Operation object if ShootState already exists", func() {
				expectedShootState := shootState.DeepCopy()
				expectedShootState.SetAnnotations(map[string]string{"foo": "bar"})

				gomock.InOrder(
					gardenClient.EXPECT().Create(ctx, shootState).Return(apierrors.NewAlreadyExists(gr, "foo")),
					gardenClient.EXPECT().Get(ctx, kubernetesutils.Key("fakeShootNS", "fakeShootName"), gomock.AssignableToTypeOf(&gardencorev1beta1.ShootState{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.ShootState, _ ...client.GetOption) error {
						expectedShootState.DeepCopyInto(obj)
						return nil
					}),
				)

				Expect(o.EnsureShootStateExists(ctx)).To(Succeed())

				Expect(o.GetShootState()).To(Equal(expectedShootState))
			})

			It("should fail if Create returns an error other than alreadyExists", func() {
				gomock.InOrder(
					gardenClient.EXPECT().Create(ctx, shootState).Return(fakeErr),
				)

				Expect(o.EnsureShootStateExists(ctx)).To(Equal(fakeErr))
			})
		})

		Describe("#DeleteShootState", func() {
			It("should add deletion confirmation and delete", func() {
				var (
					now     time.Time
					mockNow = mocktime.NewMockNow(ctrl)
				)
				mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()
				defer test.WithVars(
					&gardener.TimeNow, mockNow.Do,
				)()

				shootState.Annotations = map[string]string{gardener.ConfirmationDeletion: "true", v1beta1constants.GardenerTimestamp: now.UTC().String()}
				gomock.InOrder(
					gardenClient.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any()),
					gardenClient.EXPECT().Delete(ctx, shootState).Return(nil),
				)
				Expect(o.DeleteShootState(ctx)).To(Succeed())
			})

			It("should succeed if ShootState is already deleted", func() {
				gomock.InOrder(
					gardenClient.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any()).Return(apierrors.NewNotFound(gr, "foo")),
				)
				Expect(o.DeleteShootState(ctx)).To(Succeed())
			})

			It("should fail if patch returns an error other than NotFound", func() {
				gomock.InOrder(
					gardenClient.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any()).Return(fakeErr),
				)
				Expect(o.DeleteShootState(ctx)).To(Equal(fakeErr))
			})

			It("should fail if Delete returns an error other than NotFound", func() {
				gomock.InOrder(
					gardenClient.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any()),
					gardenClient.EXPECT().Delete(gomock.Any(), gomock.Any()).Return(fakeErr),
				)
				Expect(o.DeleteShootState(ctx)).To(Equal(fakeErr))
			})
		})

		Describe("#GetShootState", func() {
			It("should not panic if ShootState was not stored", func() {
				Expect(o.GetShootState()).To(BeNil())
			})

			It("should return the correct ShootState", func() {
				o.SetShootState(shootState)
				Expect(o.GetShootState()).To(Equal(shootState))
			})
		})
	})

	Describe("#SaveGardenerResourcesInShootState", func() {
		var (
			o            *Operation
			ctrl         *gomock.Controller
			gardenClient *mockclient.MockClient
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			gardenClient = mockclient.NewMockClient(ctrl)
			o = &Operation{
				GardenClient: gardenClient,
			}
			shootState := &gardencorev1beta1.ShootState{
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "1",
				},
			}
			o.SetShootState(shootState)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should save the gardener resource list in the shootstate", func() {
			gardenerResourceList := v1beta1helper.GardenerResourceDataList{
				{
					Name: "test",
					Type: "test",
					Data: runtime.RawExtension{Raw: []byte(`{}`)},
				},
			}

			shootState := o.GetShootState().DeepCopy()
			shootState.Spec.Gardener = gardenerResourceList
			test.EXPECTPatch(ctx, gardenClient, shootState, o.GetShootState(), types.StrategicMergePatchType)

			Expect(
				o.SaveGardenerResourceDataInShootState(
					ctx,
					func(gardenerResources *[]gardencorev1beta1.GardenerResourceData) error {
						*gardenerResources = gardenerResourceList
						return nil
					},
				)).To(Succeed())
			Expect(o.GetShootState().Spec.Gardener).To(BeEquivalentTo(gardenerResourceList))
		})
	})

	Describe("#ToAdvertisedAddresses", func() {
		var operation *Operation

		BeforeEach(func() {
			operation = &Operation{
				Shoot: &shootpkg.Shoot{},
			}
		})

		It("returns empty list when shoot is nil", func() {
			operation.Shoot = nil

			Expect(operation.ToAdvertisedAddresses()).To(BeNil())
		})
		It("returns external address", func() {
			operation.Shoot.ExternalClusterDomain = pointer.String("foo.bar")

			addresses := operation.ToAdvertisedAddresses()

			Expect(addresses).To(HaveLen(1))
			Expect(addresses).To(ConsistOf(gardencorev1beta1.ShootAdvertisedAddress{
				Name: "external",
				URL:  "https://api.foo.bar",
			}))
		})

		It("returns internal address", func() {
			operation.Shoot.InternalClusterDomain = "baz.foo"

			addresses := operation.ToAdvertisedAddresses()

			Expect(addresses).To(HaveLen(1))
			Expect(addresses).To(ConsistOf(gardencorev1beta1.ShootAdvertisedAddress{
				Name: "internal",
				URL:  "https://api.baz.foo",
			}))
		})

		It("returns unmanaged address", func() {
			operation.APIServerAddress = "bar.foo"

			addresses := operation.ToAdvertisedAddresses()

			Expect(addresses).To(HaveLen(1))
			Expect(addresses).To(ConsistOf(gardencorev1beta1.ShootAdvertisedAddress{
				Name: "unmanaged",
				URL:  "https://bar.foo",
			}))
		})

		It("returns external and internal addresses in correct order", func() {
			operation.Shoot.ExternalClusterDomain = pointer.String("foo.bar")
			operation.Shoot.InternalClusterDomain = "baz.foo"
			operation.APIServerAddress = "bar.foo"

			addresses := operation.ToAdvertisedAddresses()

			Expect(addresses).To(Equal([]gardencorev1beta1.ShootAdvertisedAddress{
				{
					Name: "external",
					URL:  "https://api.foo.bar",
				}, {
					Name: "internal",
					URL:  "https://api.baz.foo",
				},
			}))
		})
	})
})
