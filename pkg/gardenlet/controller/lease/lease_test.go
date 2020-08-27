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

package lease

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/utils/pointer"

	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	fakeclientset "github.com/gardener/gardener/pkg/client/kubernetes/fake"
)

var _ = Describe("LeaseController", func() {
	var (
		fakeNowFunc = func() time.Time {
			return time.Date(2020, time.April, 1, 1, 1, 1, 1, time.Local)
		}
		ctrl          *gomock.Controller
		k8sClientSet  *fake.Clientset
		fakeClientMap *fakeclientmap.ClientMap

		holderUID          types.UID = "test-holderUID"
		holderName                   = "test-holderName"
		testLeaseNamespace           = "test-lease-namespace"

		lease = &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      holderName,
				Namespace: testLeaseNamespace,
			},
			Spec: coordinationv1.LeaseSpec{
				HolderIdentity:       pointer.StringPtr(holderName),
				LeaseDurationSeconds: pointer.Int32Ptr(2),
			},
		}

		ownerRef = metav1.OwnerReference{
			APIVersion: "apiVersion", Name: holderName, Kind: "kind", UID: holderUID}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		k8sClientSet = fake.NewSimpleClientset()
		fakeClientMap = fakeclientmap.NewClientMap().AddClient(keys.ForGarden(), fakeclientset.NewClientSetBuilder().WithKubernetes(k8sClientSet).Build())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#leaseSync", func() {
		It("should fail if get garden client fails", func() {
			fakeClientMap = fakeclientmap.NewClientMap()

			leaseController := NewLeaseController(fakeNowFunc, fakeClientMap, 2, testLeaseNamespace)
			err := leaseController.Sync(holderName)

			Expect(err).To(HaveOccurred())
		})

		It("should not fail as clientset is set", func() {
			leaseController := NewLeaseController(fakeNowFunc, fakeClientMap, 2, testLeaseNamespace)
			err := leaseController.Sync(holderName)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should create new lease without ownerRef", func() {
			leaseController := NewLeaseController(fakeNowFunc, fakeClientMap, 2, testLeaseNamespace)

			Expect(leaseController.Sync(holderName)).NotTo(HaveOccurred())

			lease, err := k8sClientSet.CoordinationV1().Leases(testLeaseNamespace).Get(context.Background(), holderName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(lease).ShouldNot(BeNil())
			Expect(lease.Spec.RenewTime).To(BeEquivalentTo(&metav1.MicroTime{Time: fakeNowFunc()}))
			Expect(lease.OwnerReferences).Should(BeEmpty())
		})

		It("should create new lease with ownerRef", func() {
			leaseController := NewLeaseController(fakeNowFunc, fakeClientMap, 2, testLeaseNamespace)
			Expect(leaseController.Sync(holderName, ownerRef)).NotTo(HaveOccurred())

			lease, err := k8sClientSet.CoordinationV1().Leases(testLeaseNamespace).Get(context.Background(), holderName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(lease).ShouldNot(BeNil())
			Expect(lease.Spec.RenewTime).To(BeEquivalentTo(&metav1.MicroTime{Time: fakeNowFunc()}))
			Expect(lease.OwnerReferences).Should(ContainElement(ownerRef))
		})

		It("should return error, if lease doesn't exist and creating new fails", func() {
			k8sClientSet.PrependReactor("create", "leases", func(action k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf("error")
			})

			leaseController := NewLeaseController(fakeNowFunc, fakeClientMap, 2, testLeaseNamespace)

			Expect(leaseController.Sync(holderName)).To(HaveOccurred())
		})

		It("should return error, if retreiving lease returns error", func() {
			k8sClientSet.PrependReactor("get", "leases", func(action k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf("error")
			})

			leaseController := NewLeaseController(fakeNowFunc, fakeClientMap, 2, testLeaseNamespace)

			Expect(leaseController.Sync(holderName)).To(HaveOccurred())
		})

		It("should update lease time if the lease exists", func() {
			fakeTime := &metav1.MicroTime{Time: time.Date(2020, time.April, 1, 1, 1, 1, 1, time.Local)}
			lease.Spec.RenewTime = fakeTime
			Expect(k8sClientSet.Tracker().Add(lease)).NotTo(HaveOccurred())

			leaseController := NewLeaseController(fakeNowFunc, fakeClientMap, 2, testLeaseNamespace)
			Expect(leaseController.Sync(holderName)).NotTo(HaveOccurred())

			actualLease, err := k8sClientSet.CoordinationV1().Leases(testLeaseNamespace).Get(context.Background(), holderName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(actualLease.Spec.RenewTime).To(Equal(&metav1.MicroTime{Time: fakeNowFunc()}))
		})

		It("should return error, when updates lease time and the lease exists but fails", func() {
			fakeTime := &metav1.MicroTime{Time: time.Date(2020, time.April, 1, 1, 1, 1, 1, time.Local)}
			lease.Spec.RenewTime = fakeTime
			Expect(k8sClientSet.Tracker().Add(lease)).NotTo(HaveOccurred())
			k8sClientSet.PrependReactor("update", "leases", func(action k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf("error")
			})

			leaseController := NewLeaseController(fakeNowFunc, fakeClientMap, 2, testLeaseNamespace)
			err := leaseController.Sync(holderName)

			Expect(err).To(HaveOccurred())
		})

		It("should retry to update lease if conflict is returned as error from the client", func() {
			fakeTime := &metav1.MicroTime{Time: time.Date(2020, time.April, 1, 1, 1, 1, 1, time.Local)}
			lease.Spec.RenewTime = fakeTime
			Expect(k8sClientSet.Tracker().Add(lease)).NotTo(HaveOccurred())
			k8sClientSet.PrependReactor("update", "leases", func(action k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, apierrors.NewConflict(schema.GroupResource{}, holderName, fmt.Errorf("error conflict"))
			})

			leaseController := NewLeaseController(fakeNowFunc, fakeClientMap, 2, testLeaseNamespace)
			err := leaseController.Sync(holderName)

			Expect(err).To(HaveOccurred())
		})
	})
})
