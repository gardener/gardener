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

package healthz

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/goleak"
	"k8s.io/apimachinery/pkg/util/clock"
)

var _ = Describe("Periodic", func() {
	Describe("periodicHealthz", func() {
		var (
			fakeClock     *clock.FakeClock
			p             *periodicHealthz
			resetDuration = 5 * time.Second
			ignoreCurrent goleak.Option
		)

		BeforeEach(func() {
			ignoreCurrent = goleak.IgnoreCurrent()
			fakeClock = clock.NewFakeClock(time.Now())
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
				p.Start()
				defer p.Stop()

				Expect(p.Get()).To(BeTrue())
				Expect(p.timer).NotTo(BeNil())
				Expect(p.started).To(BeTrue())
			})
		})

		Describe("#Stop", func() {
			It("should stop the manager", func() {
				p.Start()
				p.Stop()

				Expect(p.Get()).To(BeFalse())
				Expect(p.started).To(BeFalse())
				Expect(p.stopCh).To(BeClosed())
			})

			It("should not panic if called twice", func() {
				p.Start()

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
				p.Start()
				defer p.Stop()

				p.Set(true)
				Expect(p.Get()).To(BeTrue())
			})

			It("should correctly set the status to false", func() {
				p.Start()
				defer p.Stop()

				p.Set(false)
				Expect(p.Get()).To(BeFalse())
			})

			It("should not set the status to true (HealthManager not started)", func() {
				p.Set(true)
				Expect(p.Get()).To(BeFalse())
			})

			It("should not set the status to true (HealthManager already stopped)", func() {
				p.Start()
				Expect(p.Get()).To(BeTrue())
				p.Stop()
				Expect(p.Get()).To(BeFalse())

				p.Set(true)
				Expect(p.Get()).To(BeFalse())
			})

			It("should correctly set the status to false after the reset duration", func() {
				p.Start()
				defer p.Stop()

				Expect(p.Get()).To(BeTrue())
				fakeClock.Step(resetDuration)
				Eventually(p.Get).Should(BeFalse())
			})

			It("should correctly reset the timer if status is changed to true", func() {
				p.Start()
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
