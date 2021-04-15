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

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	fakeclientset "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/operation"
	operationseed "github.com/gardener/gardener/pkg/operation/seed"
	operationshoot "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/test"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

var _ = Describe("operation", func() {
	var ctx = context.TODO()

	DescribeTable("#ComputeIngressHost", func(prefix, shootName, projectName, domain string, matcher gomegatypes.GomegaMatcher) {
		var (
			seed = &gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					DNS: gardencorev1beta1.SeedDNS{
						IngressDomain: &domain,
					},
				},
			}
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name: shootName,
				},
			}
			o = &Operation{
				Seed: &operationseed.Seed{
					Info: seed,
				},
				Shoot: &operationshoot.Shoot{
					Info: shoot,
				},
			}
		)

		shoot.Status = gardencorev1beta1.ShootStatus{
			TechnicalID: operationshoot.ComputeTechnicalID(projectName, shoot),
		}

		Expect(o.ComputeIngressHost(prefix)).To(matcher)
	},
		Entry("ingress calculation",
			"t",
			"fooShoot",
			"barProject",
			"ingress.seed.example.com",
			Equal("t-barProject--fooShoot.ingress.seed.example.com"),
		),
	)

	Describe("#EnsureShootStateExists", func() {
		var (
			shootState, shootStatePatched *gardencorev1alpha1.ShootState
			shoot                         *gardencorev1beta1.Shoot
			ctrl                          *gomock.Controller
			gardenClient                  *mock.MockInterface
			k8sGardenRuntimeClient        *mockclient.MockClient
			o                             *Operation
		)

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			}
			shootState = &gardencorev1alpha1.ShootState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shoot.Name,
					Namespace: shoot.Namespace,
				},
			}

			shootStatePatched = shootState.DeepCopy()
			shootStatePatched.OwnerReferences = []metav1.OwnerReference{{
				APIVersion:         "core.gardener.cloud/v1beta1",
				Kind:               "Shoot",
				Name:               shoot.Name,
				BlockOwnerDeletion: pointer.BoolPtr(false),
				Controller:         pointer.BoolPtr(true),
			}}

			ctrl = gomock.NewController(GinkgoT())
			gardenClient = mock.NewMockInterface(ctrl)
			k8sGardenRuntimeClient = mockclient.NewMockClient(ctrl)

			o = &Operation{
				K8sGardenClient: gardenClient,
				Shoot: &operationshoot.Shoot{
					Info: shoot,
				},
			}
		})

		It("should patch the ShootState's owner reference and add it to the Operation struct", func() {
			gomock.InOrder(
				gardenClient.EXPECT().Client().Return(k8sGardenRuntimeClient),
				test.EXPECTPatch(ctx, k8sGardenRuntimeClient, shootStatePatched, shootState, types.StrategicMergePatchType),
			)

			Expect(o.EnsureShootStateExists(ctx)).To(Succeed())
			Expect(o.ShootState).ToNot(BeNil())
		})

		It("should create ShootState with correct ownerReferences and add it to the Operation struct", func() {
			gomock.InOrder(
				gardenClient.EXPECT().Client().Return(k8sGardenRuntimeClient),
				test.EXPECTPatch(ctx, k8sGardenRuntimeClient, shootStatePatched, shootState, types.StrategicMergePatchType, apierrors.NewNotFound(schema.GroupResource{}, "")),
				k8sGardenRuntimeClient.EXPECT().Create(ctx, shootStatePatched),
			)

			Expect(o.EnsureShootStateExists(ctx)).To(Succeed())

			Expect(o.ShootState).ToNot(BeNil())
			Expect(len(o.ShootState.OwnerReferences)).To(Equal(1))
			Expect(o.ShootState.OwnerReferences[0].Name).To(Equal("foo"))
			Expect(o.ShootState.OwnerReferences[0].Kind).To(Equal("Shoot"))
			Expect(o.ShootState.OwnerReferences[0].BlockOwnerDeletion).ToNot(BeNil())
			Expect(*o.ShootState.OwnerReferences[0].BlockOwnerDeletion).To(BeFalse())
			Expect(o.ShootState.OwnerReferences[0].Controller).ToNot(BeNil())
			Expect(*o.ShootState.OwnerReferences[0].Controller).To(BeTrue())
		})
	})

	Describe("#SaveGardenerResourcesInShootState", func() {
		var (
			o                      *Operation
			ctrl                   *gomock.Controller
			k8sGardenRuntimeClient *mockclient.MockClient
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			k8sGardenRuntimeClient = mockclient.NewMockClient(ctrl)
			o = &Operation{
				K8sGardenClient: fakeclientset.NewClientSetBuilder().WithClient(k8sGardenRuntimeClient).WithDirectClient(k8sGardenRuntimeClient).Build(),
				ShootState:      &gardencorev1alpha1.ShootState{},
			}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should save the gardener resource list in the shootstate", func() {
			gardenerResourceList := gardencorev1alpha1helper.GardenerResourceDataList{
				{
					Name: "test",
					Type: "test",
					Data: runtime.RawExtension{Raw: []byte(`{}`)},
				},
			}

			shootState := o.ShootState.DeepCopy()
			shootState.Spec.Gardener = gardenerResourceList
			test.EXPECTPatch(ctx, k8sGardenRuntimeClient, shootState, o.ShootState, types.MergePatchType)

			Expect(o.SaveGardenerResourcesInShootState(ctx, gardenerResourceList)).To(Succeed())
			Expect(o.ShootState.Spec.Gardener).To(BeEquivalentTo(gardenerResourceList))
		})
	})
})
