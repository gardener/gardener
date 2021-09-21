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

	. "github.com/gardener/gardener/extensions/pkg/controller/common"
	mockcommon "github.com/gardener/gardener/extensions/pkg/controller/common/mock"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/clock"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ttl = 10 * time.Minute
)

var _ = Describe("WatchdogManager", func() {
	var (
		ctrl            *gomock.Controller
		c               *mockclient.MockClient
		watchdogFactory *mockcommon.MockWatchdogFactory
		watchdog        *mockcommon.MockWatchdog
		fakeClock       *clock.FakeClock
		ctx             context.Context
		watchdogManager WatchdogManager
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		watchdogFactory = mockcommon.NewMockWatchdogFactory(ctrl)
		watchdog = mockcommon.NewMockWatchdog(ctrl)
		fakeClock = clock.NewFakeClock(time.Now())
		ctx = context.TODO()
		watchdogManager = NewWatchdogManager(watchdogFactory, ttl, fakeClock, log.Log.WithName("test"))
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#GetResultAndContext", func() {
		It("should start the watchdog and return true if the watchdog creation succeeded and its result is true", func() {
			watchdogFactory.EXPECT().NewWatchdog(ctx, c, namespace, shootName).Return(watchdog, nil)
			watchdog.EXPECT().Start(context.Background())
			watchdog.EXPECT().Result().Return(true, nil)
			newCtx := context.WithValue(ctx, struct{}{}, "test")
			watchdog.EXPECT().AddContext(ctx, key).Return(newCtx, true)
			watchdog.EXPECT().RemoveContext(key).Return(true)
			watchdog.EXPECT().Stop()

			result, resultCtx, cleanup, err := watchdogManager.GetResultAndContext(ctx, c, namespace, shootName, key)
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(BeTrue())
			Expect(resultCtx).To(Equal(newCtx))
			Expect(cleanup).To(Not(BeNil()))
			cleanup()

			// Advance the time to allow the manager to remove the watchdog and call its Stop method
			fakeClock.Step(ttl)
		})

		It("should not start the watchdog and return true if the watchdog factory returned a nil watchdog", func() {
			watchdogFactory.EXPECT().NewWatchdog(ctx, c, namespace, shootName).Return(nil, nil)

			result, resultCtx, cleanup, err := watchdogManager.GetResultAndContext(ctx, c, namespace, shootName, key)
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(BeTrue())
			Expect(resultCtx).To(Equal(ctx))
			Expect(cleanup).To(BeNil())
		})

		It("should fail if the watchdog creation failed", func() {
			watchdogFactory.EXPECT().NewWatchdog(ctx, c, namespace, shootName).Return(nil, errors.New("test"))

			_, _, _, err := watchdogManager.GetResultAndContext(ctx, c, namespace, shootName, key)
			Expect(err).To(HaveOccurred())
		})

		It("should start the watchdog and return false if the watchdog creation succeeded and its result is false", func() {
			watchdogFactory.EXPECT().NewWatchdog(ctx, c, namespace, shootName).Return(watchdog, nil)
			watchdog.EXPECT().Start(context.Background())
			watchdog.EXPECT().Result().Return(false, nil)
			watchdog.EXPECT().Stop()

			result, resultCtx, cleanup, err := watchdogManager.GetResultAndContext(ctx, c, namespace, shootName, key)
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(BeFalse())
			Expect(resultCtx).To(Equal(ctx))
			Expect(cleanup).To(BeNil())

			// Advance the time to allow the manager to remove the watchdog and call its Stop method
			fakeClock.Step(ttl)
		})

		It("should start the watchdog and fail if the watchdog creation succeeded and its result is error", func() {
			watchdogFactory.EXPECT().NewWatchdog(ctx, c, namespace, shootName).Return(watchdog, nil)
			watchdog.EXPECT().Start(context.Background())
			watchdog.EXPECT().Result().Return(false, errors.New("test"))
			watchdog.EXPECT().Stop()

			_, _, _, err := watchdogManager.GetResultAndContext(ctx, c, namespace, shootName, key)
			Expect(err).To(HaveOccurred())

			// Advance the time to allow the manager to remove the watchdog and call its Stop method
			fakeClock.Step(ttl)
		})
	})
})
