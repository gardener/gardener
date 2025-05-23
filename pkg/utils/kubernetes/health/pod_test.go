// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("Pod", func() {
	DescribeTable("#CheckPod",
		func(pod *corev1.Pod, matcher types.GomegaMatcher) {
			err := health.CheckPod(pod)
			Expect(err).To(matcher)
		},

		Entry("pending", &corev1.Pod{
			Status: corev1.PodStatus{
				Phase: corev1.PodPending,
			},
		}, HaveOccurred()),
		Entry("running", &corev1.Pod{
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
			},
		}, BeNil()),
		Entry("succeeded", &corev1.Pod{
			Status: corev1.PodStatus{
				Phase: corev1.PodSucceeded,
			},
		}, BeNil()),
	)

	DescribeTable("#IsPodStale",
		func(reason string, matcher types.GomegaMatcher) {
			Expect(health.IsPodStale(reason)).To(matcher)
		},

		Entry("Evicted", "Evicted", BeTrue()),
		Entry("OutOfCpu", "OutOfCpu", BeTrue()),
		Entry("OutOfMemory", "OutOfMemory", BeTrue()),
		Entry("OutOfDisk", "OutOfDisk", BeTrue()),
		Entry("NodeAffinity", "NodeAffinity", BeTrue()),
		Entry("NodeLost", "NodeLost", BeTrue()),
		Entry("Foo", "Foo", BeFalse()),
	)

	DescribeTable("#IsPodCompleted",
		func(conditions []corev1.PodCondition, matcher types.GomegaMatcher) {
			Expect(health.IsPodCompleted(conditions)).To(matcher)
		},

		Entry("No conditions", nil, BeFalse()),
		Entry("No ready condition", []corev1.PodCondition{{}}, BeFalse()),
		Entry("Not completed", []corev1.PodCondition{{Type: "Ready", Status: "False", Reason: "Failed"}}, BeFalse()),
		Entry("Completed", []corev1.PodCondition{{Type: "Ready", Status: "False", Reason: "PodCompleted"}}, BeTrue()),
	)
})
