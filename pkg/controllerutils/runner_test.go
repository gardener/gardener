// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerutils_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	. "github.com/gardener/gardener/pkg/controllerutils"
)

var _ = Describe("Runner", func() {
	var (
		startedBootstrapRunnables []int
		addedActualRunnables      []int

		bootstrapRunnable1 manager.Runnable
		bootstrapRunnable2 manager.Runnable
		bootstrapRunnable3 manager.Runnable

		actualRunnable1 manager.Runnable
		actualRunnable2 manager.Runnable
		actualRunnable3 manager.Runnable
	)

	BeforeEach(func() {
		startedBootstrapRunnables = make([]int, 0, 3)
		addedActualRunnables = make([]int, 0, 3)

		bootstrapRunnable1 = newTestRunnable(&startedBootstrapRunnables, 1)
		bootstrapRunnable2 = newTestRunnable(&startedBootstrapRunnables, 2)
		bootstrapRunnable3 = newTestRunnable(&startedBootstrapRunnables, 3)

		actualRunnable1 = newTestRunnable(&addedActualRunnables, 1)
		actualRunnable2 = newTestRunnable(&addedActualRunnables, 2)
		actualRunnable3 = newTestRunnable(&addedActualRunnables, 3)
	})

	Describe("ControlledRunner", func() {
		var controlledRunner *ControlledRunner

		Describe("#Start", func() {
			BeforeEach(func() {
				controlledRunner = &ControlledRunner{Manager: &fakeManager{}}
			})

			It("should start and add the runnables in the correct order", func() {
				controlledRunner.BootstrapRunnables = append(controlledRunner.BootstrapRunnables,
					bootstrapRunnable3,
					bootstrapRunnable1,
					bootstrapRunnable2,
				)
				controlledRunner.ActualRunnables = append(controlledRunner.ActualRunnables,
					actualRunnable2,
					actualRunnable3,
					actualRunnable1,
				)

				Expect(controlledRunner.Start(context.TODO())).To(Succeed())

				Expect(startedBootstrapRunnables).To(Equal([]int{3, 1, 2}))
				Expect(addedActualRunnables).To(Equal([]int{2, 3, 1}))
			})
		})
	})

	Describe("#AddAllRunnables", func() {
		It("should add the runnables in the correct order", func() {
			Expect(AddAllRunnables(&fakeManager{},
				actualRunnable1,
				actualRunnable3,
				actualRunnable2,
			)).To(Succeed())
			Expect(addedActualRunnables).To(Equal([]int{1, 3, 2}))
		})

		It("should return an error when adding fails", func() {
			Expect(AddAllRunnables(&fakeManager{fail: true}, actualRunnable1)).To(MatchError(ContainSubstring("failed adding runnable to manager")))
		})
	})
})

func newTestRunnable(tracker *[]int, number int) manager.Runnable {
	return manager.RunnableFunc(func(_ context.Context) error {
		*tracker = append(*tracker, number)
		return nil
	})
}

type fakeManager struct {
	manager.Manager

	fail bool
}

func (f *fakeManager) Add(r manager.Runnable) error {
	if f.fail {
		return errors.New("failed")
	}
	return r.Start(context.TODO())
}
