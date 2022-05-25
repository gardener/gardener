// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://wwr.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common_test

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	testclock "k8s.io/utils/clock/testing"

	. "github.com/gardener/gardener/extensions/pkg/controller/common"
)

const (
	interval = 30 * time.Second
	timeout  = 50 * time.Millisecond
	key      = "foo"
)

var _ = Describe("CheckerWatchdog", func() {
	var (
		ctrl      *gomock.Controller
		checker   *fakeChecker
		fakeClock *testclock.FakeClock
		ctx       context.Context
		watchdog  Watchdog
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		checker = &fakeChecker{called: make(chan struct{}, 2)} // we expect 2 calls in every test case
		fakeClock = testclock.NewFakeClock(time.Now())
		ctx = context.TODO()
		watchdog = NewCheckerWatchdog(checker, interval, timeout, fakeClock, logr.Discard())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Start / #Stop / #AddContext / #RemoveContext / #Result", func() {
		It("should not cancel the context returned by AddContext if the checker returns true", func() {
			checker.result = true

			watchdog.Start(ctx)
			defer watchdog.Stop()
			newCtx, firstAdded := watchdog.AddContext(ctx, key)
			Expect(newCtx).To(Not(Equal(ctx)))
			Expect(firstAdded).To(BeTrue())
			defer func() {
				lastRemoved := watchdog.RemoveContext(key)
				Expect(lastRemoved).To(BeTrue())
			}()
			result, err := watchdog.Result()
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(BeTrue())
			Eventually(checker.called).Should(Receive())
			fakeClock.Step(interval)
			result2, err := watchdog.Result()
			Expect(err).To(Not(HaveOccurred()))
			Expect(result2).To(BeTrue())
			Eventually(checker.called).Should(Receive())
			Consistently(newCtx.Done()).Should(Not(BeClosed()))
		})

		It("should cancel the context returned by AddContext if the checker returns false", func() {
			checker.result = false

			watchdog.Start(ctx)
			defer watchdog.Stop()
			newCtx, firstAdded := watchdog.AddContext(ctx, key)
			Expect(newCtx).To(Not(Equal(ctx)))
			Expect(firstAdded).To(BeTrue())
			defer func() {
				lastRemoved := watchdog.RemoveContext(key)
				Expect(lastRemoved).To(BeTrue())
			}()
			result, err := watchdog.Result()
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(BeFalse())
			Eventually(checker.called).Should(Receive())
			fakeClock.Step(interval)
			result2, err := watchdog.Result()
			Expect(err).To(Not(HaveOccurred()))
			Expect(result2).To(BeFalse())
			Eventually(checker.called).Should(Receive())
			Eventually(newCtx.Done()).Should(BeClosed())
		})

		It("should cancel the context returned by AddContext if the checker returns an error", func() {
			checker.err = errors.New("text")

			watchdog.Start(ctx)
			defer watchdog.Stop()
			newCtx, firstAdded := watchdog.AddContext(ctx, key)
			Expect(newCtx).To(Not(Equal(ctx)))
			Expect(firstAdded).To(BeTrue())
			defer func() {
				lastRemoved := watchdog.RemoveContext(key)
				Expect(lastRemoved).To(BeTrue())
			}()
			_, err := watchdog.Result()
			Expect(err).To(HaveOccurred())
			Eventually(checker.called).Should(Receive())
			fakeClock.Step(interval)
			_, err = watchdog.Result()
			Expect(err).To(HaveOccurred())
			Eventually(checker.called).Should(Receive())
			Eventually(newCtx.Done()).Should(BeClosed())
		})

		It("should cancel the context returned by AddContext if the checker times out", func() {
			checker.err = context.DeadlineExceeded

			watchdog.Start(ctx)
			defer watchdog.Stop()
			newCtx, firstAdded := watchdog.AddContext(ctx, key)
			Expect(newCtx).To(Not(Equal(ctx)))
			Expect(firstAdded).To(BeTrue())
			defer func() {
				lastRemoved := watchdog.RemoveContext(key)
				Expect(lastRemoved).To(BeTrue())
			}()
			_, err := watchdog.Result()
			Expect(err).To(HaveOccurred())
			Eventually(checker.called).Should(Receive())
			fakeClock.Step(interval)
			_, err = watchdog.Result()
			Expect(err).To(HaveOccurred())
			Eventually(checker.called).Should(Receive())
			Eventually(newCtx.Done()).Should(BeClosed())
		})
	})
})

type fakeChecker struct {
	// Check will return these
	result bool
	err    error

	// each Check invocation will send one notification to this channel, buffer accordingly!
	called chan struct{}
}

func (f fakeChecker) Check(ctx context.Context) (bool, error) {
	f.called <- struct{}{}
	return f.result, f.err
}
