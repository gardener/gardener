// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package heartbeat

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Extensions Heartbeat Controller tests", func() {
	var lease *coordinationv1.Lease

	BeforeEach(func() {
		fakeClock.SetTime(time.Now())

		lease = &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener-extension-heartbeat",
				Namespace: testNamespace.Name,
			},
		}
	})

	It("should create heartbeat lease resource and keep it updated", func() {
		By("Wait until heartbeat lease resource is created")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(lease), lease)).To(Succeed())
			g.Expect(lease.Spec.LeaseDurationSeconds).To(PointTo(Equal(int32(1))))
			g.Expect(lease.Spec.HolderIdentity).To(PointTo(Equal(testID)))
			g.Expect(lease.Spec.RenewTime.Equal(&metav1.MicroTime{Time: fakeClock.Now().Truncate(time.Microsecond)})).To(BeTrue())
		}).Should(Succeed())

		By("Step fake clock")
		fakeClock.Step(time.Second)

		By("Wait until heartbeat lease's RenewTime is updated")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(lease), lease)).To(Succeed())
			g.Expect(lease.Spec.RenewTime.Equal(&metav1.MicroTime{Time: fakeClock.Now().Truncate(time.Microsecond)})).To(BeTrue())
		}).Should(Succeed())
	})
})
