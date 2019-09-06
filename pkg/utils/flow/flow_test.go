// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	mockflow "github.com/gardener/gardener/pkg/mock/gardener/utils/flow"
	"github.com/golang/mock/gomock"
	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"errors"
	"sync"
	"testing"

	"github.com/gardener/gardener/pkg/utils/flow"
)

func TestUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Flow Suite")
}

type AtomicStringList struct {
	lock   sync.RWMutex
	values []string
}

func NewAtomicStringList() *AtomicStringList {
	return &AtomicStringList{}
}

func (a *AtomicStringList) Append(values ...string) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.values = append(a.values, values...)
}

func (a *AtomicStringList) Values() []string {
	a.lock.RLock()
	defer a.lock.RUnlock()

	if a.values == nil {
		return nil
	}

	out := make([]string, len(a.values))
	copy(out, a.values)
	return out
}

var _ = Describe("Flow", func() {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	var (
		ctrl *gomock.Controller
	)
	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})
	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Run", func() {
		It("should execute in the correct sequence", func() {
			list := NewAtomicStringList()
			mkListAppender := func(value string) flow.TaskFn {
				return func(ctx context.Context) error {
					list.Append(value)
					return nil
				}
			}

			var (
				g  = flow.NewGraph("foo")
				x1 = g.Add(flow.Task{Name: "x1", Fn: mkListAppender("x1")})
				x2 = g.Add(flow.Task{Name: "x2", Fn: mkListAppender("x2")})
				y1 = g.Add(flow.Task{Name: "y1", Fn: mkListAppender("y1"), Dependencies: flow.NewTaskIDs(x1, x2)})
				y2 = g.Add(flow.Task{Name: "y2", Fn: mkListAppender("y2"), Dependencies: flow.NewTaskIDs(x1, x2)})
				z1 = g.Add(flow.Task{Name: "z1", Fn: mkListAppender("z1"), Dependencies: flow.NewTaskIDs(y1, y2)})
				_  = g.Add(flow.Task{Name: "z2", Fn: mkListAppender("z2"), Dependencies: flow.NewTaskIDs(y1, y2, z1)})
				f  = g.Compile()
			)

			Expect(f.Run(flow.Opts{})).ToNot(HaveOccurred())
			values := list.Values()
			Expect(values).To(HaveLen(6))
			Expect(values[0:2]).To(ConsistOf("x1", "x2"))
			Expect(values[2:4]).To(ConsistOf("y1", "y2"))
			Expect(values[4]).To(Equal("z1"))
			Expect(values[5]).To(Equal("z2"))
		})

		It("should yield the correct errors", func() {
			var (
				err1 = errors.New("err1")
				err2 = errors.New("err2")

				g = flow.NewGraph("foo")
				_ = g.Add(flow.Task{Name: "x", Fn: func(ctx context.Context) error { return err1 }})
				_ = g.Add(flow.Task{Name: "y", Fn: func(ctx context.Context) error { return err2 }})
				f = g.Compile()
			)

			err := f.Run(flow.Opts{})
			Expect(err).To(HaveOccurred())
			causes := flow.Causes(err)
			Expect(causes.Errors).To(ConsistOf(err1, err2))
		})

		It("should not process any function due to a canceled context", func() {
			var (
				g = flow.NewGraph("foo")
				_ = g.Add(flow.Task{Name: "x", Fn: func(ctx context.Context) error {
					Fail("Task has been called")
					return nil
				}})
				f = g.Compile()
			)

			err := f.Run(flow.Opts{Context: canceledCtx})
			Expect(err).To(HaveOccurred())
			Expect(flow.WasCanceled(err)).To(BeTrue())
		})

		It("should stop the execution after the context has been canceled in between tasks", func() {
			var (
				g           = flow.NewGraph("foo")
				ctx, cancel = context.WithCancel(context.Background())
				x           = g.Add(flow.Task{Name: "x", Fn: func(ctx context.Context) error {
					cancel()
					return nil
				}})
				_ = g.Add(flow.Task{Name: "y", Fn: func(ctx context.Context) error {
					Fail("Task has been called")
					return nil
				}, Dependencies: flow.NewTaskIDs(x)})
				f = g.Compile()
			)
			// prevent leakage
			defer cancel()

			err := f.Run(flow.Opts{Context: ctx})
			Expect(err).To(HaveOccurred())
			Expect(flow.WasCanceled(err)).To(BeTrue())
		})
	})

	Describe("#Sequential", func() {
		It("should run the given functions in sequence", func() {
			var (
				ctx = context.TODO()
				f1  = mockflow.NewMockTaskFn(ctrl)
				f2  = mockflow.NewMockTaskFn(ctrl)
			)

			gomock.InOrder(
				f1.EXPECT().Do(ctx),
				f2.EXPECT().Do(ctx),
			)

			Expect(flow.Sequential(f1.Do, f2.Do)(ctx)).To(Succeed())
		})

		It("should error if one of the functions errors", func() {
			var (
				ctx         = context.TODO()
				expectedErr = errors.New("err")
				f1          = mockflow.NewMockTaskFn(ctrl)
				f2          = mockflow.NewMockTaskFn(ctrl)
			)

			f1.EXPECT().Do(ctx).Return(expectedErr)

			err := flow.Sequential(f1.Do, f2.Do)(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeIdenticalTo(expectedErr))
		})

		It("should cancel the execution in between the calls if the context is expired", func() {
			var (
				ctx, cancel = context.WithCancel(context.Background())
				f1          = mockflow.NewMockTaskFn(ctrl)
				f2          = mockflow.NewMockTaskFn(ctrl)
			)
			defer cancel()

			f1.EXPECT().Do(ctx).Do(func(context.Context) {
				cancel()
			})

			err := flow.Sequential(f1.Do, f2.Do)(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(ctx.Err()))
		})
	})

	Describe("#Parallel", func() {
		It("should execute the functions in parallel", func() {
			var (
				ctx = context.TODO()
				f1  = mockflow.NewMockTaskFn(ctrl)
				f2  = mockflow.NewMockTaskFn(ctrl)
				f3  = mockflow.NewMockTaskFn(ctrl)
			)

			f1.EXPECT().Do(ctx)
			f2.EXPECT().Do(ctx)
			f3.EXPECT().Do(ctx)

			Expect(flow.Parallel(f1.Do, f2.Do, f3.Do)(ctx)).To(Succeed())
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
})
