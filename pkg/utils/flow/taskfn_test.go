// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package flow_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/gardener/gardener/pkg/utils/flow"
	mockflow "github.com/gardener/gardener/pkg/utils/flow/mock"
)

var _ = Describe("task functions", func() {
	var (
		ctrl          *gomock.Controller
		ignoreCurrent goleak.Option
	)
	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ignoreCurrent = goleak.IgnoreCurrent()
	})
	AfterEach(func() {
		ctrl.Finish()
		goleak.VerifyNone(GinkgoT(), ignoreCurrent)
	})

	Describe("#Parallel", func() {
		It("should execute the functions in parallel", func() {
			var (
				allTasksStarted = make(chan struct{})
				started         = make(chan struct{}, 3)

				ctx = context.Background()
				fn  = flow.TaskFn(func(ctx context.Context) error {
					started <- struct{}{}
					// block until all tasks were started to verify parallel execution of tasks
					<-allTasksStarted
					return nil
				})
			)

			go func() {
				defer GinkgoRecover()
				Eventually(started).Should(Receive())
				Eventually(started).Should(Receive())
				Eventually(started).Should(Receive())
				close(allTasksStarted)
			}()

			Expect(flow.Parallel(fn, fn, fn)(ctx)).To(Succeed())
			Eventually(allTasksStarted).Should(BeClosed())
		})

		It("should execute the functions and collect their errors", func() {
			var (
				ctx = context.TODO()
				f1  = mockflow.NewMockTaskFn(ctrl)
				f2  = mockflow.NewMockTaskFn(ctrl)
				f3  = mockflow.NewMockTaskFn(ctrl)

				err1 = errors.New("e1")
				err2 = errors.New("e2")
			)

			f1.EXPECT().Do(ctx).Return(err1)
			f2.EXPECT().Do(ctx).Return(err2)
			f3.EXPECT().Do(ctx)

			err := flow.Parallel(f1.Do, f2.Do, f3.Do)(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(&multierror.Error{}))
			Expect(err.(*multierror.Error).Errors).To(ConsistOf(err1, err2))
		})
	})

	Describe("#ParallelExitOnError", func() {
		It("should execute the functions in parallel", func() {
			var (
				allTasksStarted = make(chan struct{})
				started         = make(chan struct{}, 3)

				ctx = context.Background()
				fn  = flow.TaskFn(func(ctx context.Context) error {
					started <- struct{}{}
					// block until all tasks were started to verify parallel execution of tasks
					<-allTasksStarted
					return nil
				})
			)

			go func() {
				defer GinkgoRecover()
				Eventually(started).Should(Receive())
				Eventually(started).Should(Receive())
				Eventually(started).Should(Receive())
				close(allTasksStarted)
			}()

			Expect(flow.ParallelExitOnError(fn, fn, fn)(ctx)).To(Succeed())
			Eventually(allTasksStarted).Should(BeClosed())
		})

		It("should exit on error and cancel parallel functions", func() {
			var (
				ctx       = context.Background()
				cancelled = make(chan struct{})

				f1 = flow.TaskFn(func(ctx context.Context) error {
					return fmt.Errorf("task1")
				})
				f2 = flow.TaskFn(func(ctx context.Context) error {
					<-ctx.Done()
					close(cancelled)
					return fmt.Errorf("task2")
				})
			)

			Expect(flow.ParallelExitOnError(f1, f2)(ctx)).To(MatchError("task1"))
			Eventually(cancelled).Should(BeClosed())
		})
	})
})
