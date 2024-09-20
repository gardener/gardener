// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
})
