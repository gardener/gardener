// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package podschedulername_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	. "github.com/gardener/gardener/pkg/resourcemanager/webhook/podschedulername"
)

var _ = Describe("Handler", func() {
	var (
		ctx = context.TODO()

		handler *Handler
		pod     *corev1.Pod
	)

	BeforeEach(func() {
		handler = &Handler{SchedulerName: "foo-scheduler"}
		pod = &corev1.Pod{}
	})

	Describe("#Default", func() {
		It("should not patch the scheduler name when the pod specifies custom scheduler", func() {
			schedulerName := "bar-scheduler"
			pod.Spec.SchedulerName = schedulerName

			Expect(handler.Default(ctx, pod)).To(Succeed())
			Expect(pod.Spec.SchedulerName).To(Equal(schedulerName))
		})

		It("should patch the scheduler name when the pod scheduler is not specified", func() {
			pod.Spec.SchedulerName = ""

			Expect(handler.Default(ctx, pod)).To(Succeed())
			Expect(pod.Spec.SchedulerName).To(Equal(handler.SchedulerName))
		})
	})
})
