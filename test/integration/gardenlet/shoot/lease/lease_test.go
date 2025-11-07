// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package lease_test

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	leasecontroller "github.com/gardener/gardener/pkg/gardenlet/controller/lease"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Shoot lease controller tests", func() {
	var lease *coordinationv1.Lease

	BeforeEach(OncePerOrdered, func() {
		fakeClock.SetTime(time.Now())

		lease = &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{Name: "self-hosted-shoot-" + shoot.Name, Namespace: shoot.Namespace}}
	})

	Describe("maintain the Lease object and set the internal health status to true", Ordered, func() {
		It("should ensured Lease gets maintained", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(lease), lease)).To(Succeed())
				g.Expect(lease.OwnerReferences).To(ConsistOf(metav1.OwnerReference{APIVersion: "core.gardener.cloud/v1beta1", Kind: "Shoot", Name: shoot.Name, UID: shoot.UID}))
				g.Expect(lease.Spec.RenewTime.Sub(fakeClock.Now())).To(BeNumerically("<=", 0))
				g.Expect(lease.Spec.LeaseDurationSeconds).To(PointTo(Equal(int32(2))))
				g.Expect(lease.Spec.HolderIdentity).To(Equal(&shoot.Name))
				g.Expect(healthManager.Get()).To(BeTrue())
			}).Should(Succeed())
		})

		It("should ensure GardenletReady condition gets maintained", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				g.Expect(shoot.Status.Conditions).To(ContainCondition(OfType(gardencorev1beta1.GardenletReady), WithStatus(gardencorev1beta1.ConditionTrue)))
			}).Should(Succeed())
		})

		It("should step the fake clock", func() {
			fakeClock.Step(time.Hour)
		})

		It("should ensure health status is set to true", func() {
			Consistently(func() bool {
				return healthManager.Get()
			}).Should(BeTrue())
		})

		It("should ensure Lease gets updated", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(lease), lease)).To(Succeed())
				g.Expect(lease.Spec.RenewTime.Sub(fakeClock.Now())).To(BeNumerically("<=", 0))
				g.Expect(lease.OwnerReferences).To(ConsistOf(metav1.OwnerReference{APIVersion: "core.gardener.cloud/v1beta1", Kind: "Shoot", Name: shoot.Name, UID: shoot.UID}))
				g.Expect(lease.Spec.LeaseDurationSeconds).To(PointTo(Equal(int32(2))))
				g.Expect(lease.Spec.HolderIdentity).To(Equal(&shoot.Name))
			}).Should(Succeed())
		})
	})

	Describe("do not update the Lease object and set the internal health status to false", Ordered, func() {
		var fakeError error

		BeforeEach(OncePerOrdered, func() {
			DeferCleanup(test.WithVar(&leasecontroller.CheckConnection, func(context.Context, rest.Interface) error { return fakeError }))
		})

		It("should ensure Lease gets maintained", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(lease), lease)).To(Succeed())
				g.Expect(lease.OwnerReferences).To(ConsistOf(metav1.OwnerReference{APIVersion: "core.gardener.cloud/v1beta1", Kind: "Shoot", Name: shoot.Name, UID: shoot.UID}))
				g.Expect(lease.Spec.RenewTime.Sub(fakeClock.Now())).To(BeNumerically("<=", 0))
				g.Expect(lease.Spec.LeaseDurationSeconds).To(PointTo(Equal(int32(2))))
				g.Expect(lease.Spec.HolderIdentity).To(Equal(&shoot.Name))
				g.Expect(healthManager.Get()).To(BeTrue())
			}).Should(Succeed())
		})

		It("should ensure GardenletReady condition gets maintained", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
				g.Expect(shoot.Status.Conditions).To(ContainCondition(OfType(gardencorev1beta1.GardenletReady), WithStatus(gardencorev1beta1.ConditionTrue)))
			}).Should(Succeed())
		})

		It("should make the seed connection fail", func() {
			fakeError = errors.New("fake")
		})

		It("should ensure health status gets set to false", func() {
			Eventually(func() bool {
				return healthManager.Get()
			}).Should(BeFalse())
		})

		It("should step the fake clock", func() {
			fakeClock.Step(time.Hour)
		})

		It("should ensure the Lease object is not updated and the internal health status remains false", func() {
			Consistently(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(lease), lease)).To(Succeed())
				g.Expect(fakeClock.Now().Sub(lease.Spec.RenewTime.Time)).To(BeNumerically(">=", time.Hour))
				g.Expect(lease.OwnerReferences).To(ConsistOf(metav1.OwnerReference{APIVersion: "core.gardener.cloud/v1beta1", Kind: "Shoot", Name: shoot.Name, UID: shoot.UID}))
				g.Expect(lease.Spec.LeaseDurationSeconds).To(PointTo(Equal(int32(2))))
				g.Expect(lease.Spec.HolderIdentity).To(Equal(&shoot.Name))
			}).Should(Succeed())
		})
	})
})
