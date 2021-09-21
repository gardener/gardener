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

var _ = Describe("CheckerWatchdogFactory", func() {
	var (
		ctrl                   *gomock.Controller
		c                      *mockclient.MockClient
		checkerFactory         *mockcommon.MockCheckerFactory
		checker                *mockcommon.MockChecker
		ctx                    context.Context
		checkerWatchdogFactory WatchdogFactory
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		checkerFactory = mockcommon.NewMockCheckerFactory(ctrl)
		checker = mockcommon.NewMockChecker(ctrl)
		ctx = context.TODO()
		checkerWatchdogFactory = NewCheckerWatchdogFactory(checkerFactory, interval, timeout, clock.NewFakeClock(time.Now()), log.Log.WithName("test"))
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#NewWatchdog", func() {
		It("should create a new checker watchdog if the checker creation succeeded", func() {
			checkerFactory.EXPECT().NewChecker(ctx, c, namespace, shootName).Return(checker, nil)

			watchdog, err := checkerWatchdogFactory.NewWatchdog(ctx, c, namespace, shootName)
			Expect(err).To(Not(HaveOccurred()))
			Expect(watchdog).To(Not(BeNil()))
		})

		It("should not create a checker watchdog if the checker factory returned a nil checker", func() {
			checkerFactory.EXPECT().NewChecker(ctx, c, namespace, shootName).Return(nil, nil)

			watchdog, err := checkerWatchdogFactory.NewWatchdog(ctx, c, namespace, shootName)
			Expect(err).To(Not(HaveOccurred()))
			Expect(watchdog).To(BeNil())
		})

		It("should fail if the checker creation failed", func() {
			checkerFactory.EXPECT().NewChecker(ctx, c, namespace, shootName).Return(nil, errors.New("test"))

			_, err := checkerWatchdogFactory.NewWatchdog(ctx, c, namespace, shootName)
			Expect(err).To(HaveOccurred())
		})
	})
})
