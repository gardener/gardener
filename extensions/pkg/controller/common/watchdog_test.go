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

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	interval = 10 * time.Millisecond
)

var _ = Describe("Watchdog", func() {
	var (
		ctrl     *gomock.Controller
		checker  *mockcommon.MockChecker
		ctx      context.Context
		logger   logr.Logger
		watchdog Watchdog
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		checker = mockcommon.NewMockChecker(ctrl)

		ctx = context.TODO()
		logger = log.Log.WithName("test")

		watchdog = NewCheckerWatchdog(checker, interval, logger)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	var cancelled = func(ctx context.Context) bool {
		select {
		case <-ctx.Done():
			return true
		case <-time.After(interval * 2):
			return false
		}
	}

	Describe("#Check", func() {
		It("should not cancel the context if the checker returns true", func() {
			checker.EXPECT().Check(gomock.Any()).Return(true, nil).AnyTimes()

			ctx, cancel := watchdog.Start(ctx)
			defer cancel()
			Expect(cancelled(ctx)).To(BeFalse())
		})

		It("should cancel the context if the checker returns false", func() {
			checker.EXPECT().Check(gomock.Any()).Return(false, nil).AnyTimes()

			ctx, cancel := watchdog.Start(ctx)
			defer cancel()
			Expect(cancelled(ctx)).To(BeTrue())
		})

		It("should cancel the context if the checker returns an error", func() {
			checker.EXPECT().Check(gomock.Any()).Return(false, errors.New("text")).AnyTimes()

			ctx, cancel := watchdog.Start(ctx)
			defer cancel()
			Expect(cancelled(ctx)).To(BeTrue())
		})
	})
})
