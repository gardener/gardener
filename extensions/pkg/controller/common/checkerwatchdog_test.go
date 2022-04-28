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

	"go.uber.org/zap/zapcore"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/gardener/gardener/extensions/pkg/controller/common"
	mockcommon "github.com/gardener/gardener/extensions/pkg/controller/common/mock"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/clock"
)

const (
	interval = 30 * time.Second
	timeout  = 50 * time.Millisecond
	key      = "foo"
)

var _ = Describe("CheckerWatchdog", func() {
	var (
		ctrl      *gomock.Controller
		checker   *mockcommon.MockChecker
		fakeClock *clock.FakeClock
		ctx       context.Context
		watchdog  Watchdog
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		checker = mockcommon.NewMockChecker(ctrl)
		fakeClock = clock.NewFakeClock(time.Now())
		ctx = context.TODO()
		watchdog = NewCheckerWatchdog(checker, interval, timeout, fakeClock, logzap.New(logzap.Level(zapcore.DebugLevel*2), logzap.WriteTo(GinkgoWriter)))
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Start / #Stop / #AddContext / #RemoveContext / #Result", func() {
		It("should not cancel the context returned by AddContext if the checker returns true", func() {
			checker.EXPECT().Check(gomock.Any()).Return(true, nil).Times(2)

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
			fakeClock.Step(interval)
			result2, err := watchdog.Result()
			Expect(err).To(Not(HaveOccurred()))
			Expect(result2).To(BeTrue())
			Eventually(newCtx.Done()).Should(Not(BeClosed()))
		})

		It("should cancel the context returned by AddContext if the checker returns false", func() {
			checker.EXPECT().Check(gomock.Any()).Return(false, nil).Times(2)

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
			fakeClock.Step(interval)
			result2, err := watchdog.Result()
			Expect(err).To(Not(HaveOccurred()))
			Expect(result2).To(BeFalse())
			Eventually(newCtx.Done()).Should(BeClosed())
		})

		It("should cancel the context returned by AddContext if the checker returns an error", func() {
			checker.EXPECT().Check(gomock.Any()).Return(false, errors.New("text")).Times(2)

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
			fakeClock.Step(interval)
			_, err = watchdog.Result()
			Expect(err).To(HaveOccurred())
			Eventually(newCtx.Done()).Should(BeClosed())
		})

		It("should cancel the context returned by AddContext if the checker times out", func() {
			checker.EXPECT().Check(gomock.Any()).DoAndReturn(func(ctx context.Context) (bool, error) {
				// return context.DeadlineExceeded to simulate timeout
				return false, context.DeadlineExceeded
			}).Times(2)

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
			fakeClock.Step(interval)
			_, err = watchdog.Result()
			Expect(err).To(HaveOccurred())
			Eventually(newCtx.Done()).Should(BeClosed())
		})
	})
})
