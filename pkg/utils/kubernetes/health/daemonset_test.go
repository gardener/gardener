// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("DaemonSet", func() {
	oneUnavailable := intstr.FromInt32(1)

	DescribeTable("#CheckDaemonSet",
		func(daemonSet *appsv1.DaemonSet, matcher types.GomegaMatcher) {
			err := health.CheckDaemonSet(daemonSet)
			Expect(err).To(matcher)
		},
		Entry("not observed at latest version", &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Generation: 1},
		}, HaveOccurred()),
		Entry("not enough scheduled", &appsv1.DaemonSet{
			Status: appsv1.DaemonSetStatus{DesiredNumberScheduled: 1},
		}, HaveOccurred()),
		Entry("misscheduled pods", &appsv1.DaemonSet{
			Status: appsv1.DaemonSetStatus{NumberMisscheduled: 1},
		}, HaveOccurred()),
		Entry("too many unavailable pods during rollout", &appsv1.DaemonSet{
			Spec: appsv1.DaemonSetSpec{UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &oneUnavailable,
				},
			}},
			Status: appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 3,
				CurrentNumberScheduled: 3,
				NumberUnavailable:      2,
				NumberReady:            1,
				NumberAvailable:        1,
				UpdatedNumberScheduled: 2,
			},
		}, HaveOccurred()),
		Entry("too many unavailable pods", &appsv1.DaemonSet{
			Spec: appsv1.DaemonSetSpec{UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &oneUnavailable,
				},
			}},
			Status: appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 2,
				CurrentNumberScheduled: 2,
				NumberUnavailable:      2,
				NumberReady:            0,
			},
		}, HaveOccurred()),
		Entry("healthy", &appsv1.DaemonSet{
			Status: appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 1,
				CurrentNumberScheduled: 1,
				NumberReady:            1,
			},
		}, BeNil()),
		Entry("healthy with allowed unavailable pods during rollout", &appsv1.DaemonSet{
			Spec: appsv1.DaemonSetSpec{UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &oneUnavailable,
				},
			}},
			Status: appsv1.DaemonSetStatus{
				DesiredNumberScheduled: 3,
				CurrentNumberScheduled: 3,
				NumberUnavailable:      1,
				NumberReady:            2,
				NumberAvailable:        2,
				UpdatedNumberScheduled: 1,
			},
		}, BeNil()),
	)

	Describe("IsDaemonSetProgressing", func() {
		var (
			daemonSet *appsv1.DaemonSet
		)

		BeforeEach(func() {
			daemonSet = &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 42,
				},
				Status: appsv1.DaemonSetStatus{
					ObservedGeneration:     42,
					DesiredNumberScheduled: 3,
					UpdatedNumberScheduled: 3,
				},
			}
		})

		It("should return false if it is fully rolled out", func() {
			progressing, reason := health.IsDaemonSetProgressing(daemonSet)
			Expect(progressing).To(BeFalse())
			Expect(reason).To(Equal("DaemonSet is fully rolled out"))
		})

		It("should return true if observedGeneration is outdated", func() {
			daemonSet.Status.ObservedGeneration--

			progressing, reason := health.IsDaemonSetProgressing(daemonSet)
			Expect(progressing).To(BeTrue())
			Expect(reason).To(Equal("observed generation outdated (41/42)"))
		})

		It("should return true if replicas still need to be updated", func() {
			daemonSet.Status.UpdatedNumberScheduled--

			progressing, reason := health.IsDaemonSetProgressing(daemonSet)
			Expect(progressing).To(BeTrue())
			Expect(reason).To(Equal("2 of 3 replica(s) have been updated"))
		})
	})
})
