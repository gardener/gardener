// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthz

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/goleak"
	testclock "k8s.io/utils/clock/testing"
)

var _ = Describe("Periodic", func() {
	Describe("periodicHealthz", func() {
		var (
			ctx           = context.TODO()
			fakeClock     *testclock.FakeClock
			p             *periodicHealthz
			resetDuration = 5 * time.Second
			ignoreCurrent goleak.Option
		)

		BeforeEach(func() {
			ignoreCurrent = goleak.IgnoreCurrent()
			fakeClock = testclock.NewFakeClock(time.Now())
			p = NewPeriodicHealthz(fakeClock, resetDuration).(*periodicHealthz)
		})

		AfterEach(func() {
			goleak.VerifyNone(GinkgoT(), ignoreCurrent)
		})

		Describe("#Name", func() {
			It("should return the correct name", func() {
				Expect(p.Name()).To(Equal("periodic"))
			})
		})

		Describe("#Start", func() {
			It("should start the manager", func() {
				Expect(p.Start(ctx)).To(Succeed())
				defer p.Stop()

				Expect(p.Get()).To(BeTrue())
				Expect(p.timer).NotTo(BeNil())
				Expect(p.started).To(BeTrue())
			})
		})

		Describe("#Stop", func() {
			It("should stop the manager", func() {
				Expect(p.Start(ctx)).To(Succeed())
				p.Stop()

				Expect(p.Get()).To(BeFalse())
				Expect(p.started).To(BeFalse())
				Expect(p.stopCh).To(BeClosed())
			})

			It("should not panic if called twice", func() {
				Expect(p.Start(ctx)).To(Succeed())

				Expect(func() {
					p.Stop()
					p.Stop()
				}).NotTo(Panic())

				Expect(p.Get()).To(BeFalse())
				Expect(p.started).To(BeFalse())
			})
		})

		Describe("#Set", func() {
			It("should correctly set the status to true", func() {
				Expect(p.Start(ctx)).To(Succeed())
				defer p.Stop()

				p.Set(true)
				Expect(p.Get()).To(BeTrue())
			})

			It("should correctly set the status to false", func() {
				Expect(p.Start(ctx)).To(Succeed())
				defer p.Stop()

				p.Set(false)
				Expect(p.Get()).To(BeFalse())
			})

			It("should set the status to true (HealthManager not started)", func() {
				p.Set(true)
				Expect(p.Get()).To(BeTrue())
			})

			It("should set the status to true (HealthManager already stopped)", func() {
				Expect(p.Start(ctx)).To(Succeed())
				Expect(p.Get()).To(BeTrue())
				p.Stop()
				Expect(p.Get()).To(BeFalse())

				p.Set(true)
				Expect(p.Get()).To(BeTrue())
			})

			It("should correctly set the status to false after the reset duration", func() {
				Expect(p.Start(ctx)).To(Succeed())
				defer p.Stop()

				Expect(p.Get()).To(BeTrue())
				fakeClock.Step(resetDuration)
				Eventually(p.Get).Should(BeFalse())
			})

			It("should correctly reset the timer if status is changed to true", func() {
				Expect(p.Start(ctx)).To(Succeed())
				defer p.Stop()

				Expect(p.Get()).To(BeTrue())
				fakeClock.Step(resetDuration)
				Eventually(p.Get).Should(BeFalse())

				p.Set(true)
				Expect(p.Get()).To(BeTrue())
				fakeClock.Step(resetDuration)
				Eventually(p.Get).Should(BeFalse())
			})
		})

		Describe("#Get", func() {
			It("should get the correct status (true)", func() {
				p.health = true
				Expect(p.Get()).To(BeTrue())
			})

			It("should get the correct status (false)", func() {
				p.health = false
				Expect(p.Get()).To(BeFalse())
			})
		})
	})
})
