// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthz_test

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	. "github.com/gardener/gardener/pkg/healthz"
)

var _ = Describe("Informer", func() {
	Describe("#NewCacheSyncHealthz", func() {
		It("should succeed if all informers sync", func() {
			checker := NewCacheSyncHealthz(&fakeSyncWaiter{true})
			Expect(checker(nil)).To(Succeed())
		})

		It("should fail if informers don't sync", func() {
			checker := NewCacheSyncHealthz(&fakeSyncWaiter{false})
			Expect(checker(nil)).To(MatchError(ContainSubstring("not synced")))
		})
	})

	Describe("#NewCacheSyncHealthzWithDeadline", func() {
		var (
			log       = logr.Discard()
			fakeClock *testclock.FakeClock
			deadline  = time.Minute
			checker   healthz.Checker
		)

		BeforeEach(func() {
			fakeClock = testclock.NewFakeClock(time.Now())
		})

		It("should succeed if all informers sync", func() {
			checker = NewCacheSyncHealthzWithDeadline(log, fakeClock, &fakeSyncWaiter{true}, deadline)
			Expect(checker(nil)).To(Succeed())
		})

		When("the informers are not synced", func() {
			var waiter *fakeSyncWaiter

			BeforeEach(func() {
				waiter = &fakeSyncWaiter{false}
				checker = NewCacheSyncHealthzWithDeadline(log, fakeClock, waiter, deadline)
			})

			It("should succeed as long as the deadline is not hit even if not all informers sync", func() {
				By("succeed because deadline is not hit")
				Expect(checker(nil)).To(Succeed())
				fakeClock.Step(deadline / 2)

				By("succeed because deadline is not hit")
				Expect(checker(nil)).To(Succeed())
				fakeClock.Step(deadline / 2)

				By("fail because deadline is hit")
				Expect(checker(nil)).To(MatchError(ContainSubstring("not synced")))
			})

			It("should reset the time all informers are synced after not working for a certain time", func() {
				By("succeed because deadline is not hit")
				Expect(checker(nil)).To(Succeed())
				fakeClock.Step(deadline / 2)

				By("succeed because deadline is not hit")
				Expect(checker(nil)).To(Succeed())
				fakeClock.Step(deadline / 2)

				By("succeed because caches are synced")
				waiter.value = true
				Expect(checker(nil)).To(Succeed())

				By("succeed because deadline is not hit")
				waiter.value = false
				Expect(checker(nil)).To(Succeed())
				fakeClock.Step(deadline / 2)

				By("succeed because deadline is not hit")
				Expect(checker(nil)).To(Succeed())
				fakeClock.Step(deadline / 2)

				By("fail because deadline is hit")
				Expect(checker(nil)).To(MatchError(ContainSubstring("not synced")))
			})
		})
	})
})

type fakeSyncWaiter struct {
	value bool
}

func (f *fakeSyncWaiter) WaitForCacheSync(_ context.Context) bool { return f.value }
