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
	leasereconciler "github.com/gardener/gardener/pkg/gardenlet/controller/seed/lease"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Seed lease controller tests", func() {
	var lease *coordinationv1.Lease

	BeforeEach(func() {
		fakeClock.SetTime(time.Now())

		lease = &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{Name: seed.Name, Namespace: testNamespace.Name}}
	})

	It("should maintain the Lease object and set the internal health status to true", func() {
		By("Ensure Lease got maintained")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(lease), lease)).To(Succeed())
			g.Expect(lease.OwnerReferences).To(ConsistOf(metav1.OwnerReference{APIVersion: "core.gardener.cloud/v1beta1", Kind: "Seed", Name: seed.Name, UID: seed.UID}))
			g.Expect(lease.Spec.RenewTime.Sub(fakeClock.Now())).To(BeNumerically("<=", 0))
			g.Expect(lease.Spec.LeaseDurationSeconds).To(PointTo(Equal(int32(1))))
			g.Expect(lease.Spec.HolderIdentity).To(Equal(&seed.Name))
			g.Expect(healthManager.Get()).To(BeTrue())
		}).Should(Succeed())

		By("Ensure GardenletReady condition was maintained")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
			g.Expect(seed.Status.Conditions).To(ContainCondition(OfType(gardencorev1beta1.SeedGardenletReady), WithStatus(gardencorev1beta1.ConditionTrue)))
		}).Should(Succeed())

		By("Step clock")
		fakeClock.Step(time.Hour)

		By("Ensure health status is true")
		Consistently(func() bool {
			return healthManager.Get()
		}).Should(BeTrue())

		By("Ensure Lease got updated")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(lease), lease)).To(Succeed())
			g.Expect(lease.Spec.RenewTime.Sub(fakeClock.Now())).To(BeNumerically("<=", 0))
			g.Expect(lease.OwnerReferences).To(ConsistOf(metav1.OwnerReference{APIVersion: "core.gardener.cloud/v1beta1", Kind: "Seed", Name: seed.Name, UID: seed.UID}))
			g.Expect(lease.Spec.LeaseDurationSeconds).To(PointTo(Equal(int32(1))))
			g.Expect(lease.Spec.HolderIdentity).To(Equal(&seed.Name))
		}).Should(Succeed())
	})

	It("should not update the Lease object and set the internal health status to false", func() {
		var fakeError error
		DeferCleanup(test.WithVar(&leasereconciler.CheckSeedConnection, func(context.Context, rest.Interface) error { return fakeError }))

		By("Ensure Lease got maintained")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(lease), lease)).To(Succeed())
			g.Expect(lease.OwnerReferences).To(ConsistOf(metav1.OwnerReference{APIVersion: "core.gardener.cloud/v1beta1", Kind: "Seed", Name: seed.Name, UID: seed.UID}))
			g.Expect(lease.Spec.RenewTime.Sub(fakeClock.Now())).To(BeNumerically("<=", 0))
			g.Expect(lease.Spec.LeaseDurationSeconds).To(PointTo(Equal(int32(1))))
			g.Expect(lease.Spec.HolderIdentity).To(Equal(&seed.Name))
			g.Expect(healthManager.Get()).To(BeTrue())
		}).Should(Succeed())

		By("Ensure GardenletReady condition was maintained")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
			g.Expect(seed.Status.Conditions).To(ContainCondition(OfType(gardencorev1beta1.SeedGardenletReady), WithStatus(gardencorev1beta1.ConditionTrue)))
		}).Should(Succeed())

		By("Ensure seed connection fails")
		fakeError = errors.New("fake")

		By("Ensure health status was set to false")
		Eventually(func() bool {
			return healthManager.Get()
		}).Should(BeFalse())

		By("Step clock")
		fakeClock.Step(time.Hour)

		By("Ensure Lease did not get updated")
		Consistently(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(lease), lease)).To(Succeed())
			g.Expect(fakeClock.Now().Sub(lease.Spec.RenewTime.Time)).To(BeNumerically(">=", time.Hour))
			g.Expect(lease.OwnerReferences).To(ConsistOf(metav1.OwnerReference{APIVersion: "core.gardener.cloud/v1beta1", Kind: "Seed", Name: seed.Name, UID: seed.UID}))
			g.Expect(lease.Spec.LeaseDurationSeconds).To(PointTo(Equal(int32(1))))
			g.Expect(lease.Spec.HolderIdentity).To(Equal(&seed.Name))
		}).Should(Succeed())
	})
})
