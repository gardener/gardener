// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"context"
	"errors"
	"sync"

	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/util/sets"

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
				fn  = flow.TaskFn(func(_ context.Context) error {
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
				fn  = flow.TaskFn(func(_ context.Context) error {
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

				f1 = flow.TaskFn(func(_ context.Context) error {
					return errors.New("task1")
				})
				f2 = flow.TaskFn(func(ctx context.Context) error {
					<-ctx.Done()
					close(cancelled)
					return errors.New("task2")
				})
			)

			Expect(flow.ParallelExitOnError(f1, f2)(ctx)).To(MatchError("task1"))
			Eventually(cancelled).Should(BeClosed())
		})
	})

	Describe("ParallelN", func() {
		It("should run the tasks", func() {
			var (
				ctx         = context.Background()
				n           = 2
				activeTasks = &sync.Map{}
				doneTasks   = &sync.Map{}
				blockCh     = make(chan struct{}, 5)
				doneCh      = make(chan struct{})
				fn          = func(key string) flow.TaskFn {
					return func(_ context.Context) error {
						activeTasks.Store(key, struct{}{})
						defer func() {
							doneTasks.Store(key, struct{}{})
							activeTasks.Delete(key)
						}()

						// block until unblocked by test step
						<-blockCh
						return nil
					}
				}
				taskIds    = sets.New("1", "2", "3", "4", "5")
				tasks      = []flow.TaskFn{fn("1"), fn("2"), fn("3"), fn("4"), fn("5")}
				tasksFound sets.Set[string]
			)

			go func() {
				Expect(flow.ParallelN(n, tasks...)(ctx)).To(Succeed())
				close(doneCh)
			}()

			By("Checking active tasks after initial ParallelN call")
			Eventually(func() int {
				// find two of the active tasks and remove them from the ids
				tasksFound = findTasks(taskIds, activeTasks)
				return tasksFound.Len()
			}).Should(Equal(n))
			taskIds = taskIds.Difference(tasksFound)

			By("Unblocking single task")
			blockCh <- struct{}{}
			Eventually(func() int {
				// find the newest active task and remove it from the ids
				tasksFound = findTasks(taskIds, activeTasks)
				return tasksFound.Len()
			}).Should(Equal(1))
			taskIds = taskIds.Difference(tasksFound)

			Eventually(func(g Gomega) {
				doneTasks.Range(func(key, _ any) bool {
					g.Expect(taskIds).NotTo(HaveKey(key.(string)))
					return false
				})
			}).Should(Succeed())

			By("Unblocking more tasks")
			blockCh <- struct{}{}
			blockCh <- struct{}{}
			Eventually(func() int {
				// find the remaining 2 active tasks and remove them from the ids
				tasksFound = findTasks(taskIds, activeTasks)
				return tasksFound.Len()
			}).Should(Equal(2))
			taskIds = taskIds.Difference(tasksFound)

			Eventually(func(g Gomega) {
				doneTasks.Range(func(key, _ any) bool {
					g.Expect(taskIds).NotTo(HaveKey(key.(string)))
					return false
				})
			}).Should(Succeed())

			Expect(taskIds.Len()).To(Equal(0))

			By("Unblocking remaining tasks")
			blockCh <- struct{}{}
			blockCh <- struct{}{}

			Eventually(func(g Gomega) {
				tasks := 0
				activeTasks.Range(func(_, _ any) bool {
					tasks += 1
					return true
				})
				g.Expect(tasks).To(Equal(0))
				g.Expect(doneCh).To(BeClosed())
			}).Should(Succeed())
		})

		It("should collect the errors", func() {
			var (
				ctx = context.Background()
				fn  = func(err error) flow.TaskFn {
					return func(_ context.Context) error {
						return err
					}
				}
			)
			err1 := errors.New("one")
			err2 := errors.New("two")
			err := flow.ParallelN(2, fn(err1), fn(nil), fn(err2))(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(&multierror.Error{}))
			Expect(err.(*multierror.Error).Errors).To(ConsistOf(err1, err2))
		})
	})
})

func findTasks(taskIds sets.Set[string], tasks *sync.Map) sets.Set[string] {
	tasksFound := sets.New[string]()

	tasks.Range(func(key, _ any) bool {
		if taskIds.Has(key.(string)) {
			tasksFound.Insert(key.(string))
		}
		return true
	})

	return tasksFound
}
