// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package matchers_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("BeHealthy", func() {
	var (
		matcher gomegatypes.GomegaMatcher

		pod *corev1.Pod
	)

	BeforeEach(func() {
		matcher = BeHealthy(health.CheckPod)

		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
			},
		}
	})

	When("the object is healthy", func() {
		It("should succeed", func() {
			Expect(pod).To(matcher)
		})

		It("return the correct negated failure message", func() {
			Expect(matcher.Match(pod)).To(BeTrue())
			Expect(matcher.NegatedFailureMessage(pod)).To(Equal(`Expected
    *v1.Pod bar/foo
not to be healthy but the health check did not return any error`))
		})
	})

	When("the object is not healthy", func() {
		BeforeEach(func() {
			pod.Status.Phase = corev1.PodFailed
		})

		It("should fail", func() {
			Expect(pod).NotTo(matcher)
		})

		It("return the correct failure message", func() {
			Expect(matcher.Match(pod)).To(BeFalse())
			Expect(matcher.FailureMessage(pod)).To(ContainSubstring(`Expected
    *v1.Pod bar/foo
to be healthy but the health check returned the following error:
    pod is in invalid phase "Failed"`))
		})
	})

	When("an unexpected object is passed", func() {
		It("should return an error", func() {
			Expect(matcher.Match(&appsv1.Deployment{})).Error().To(MatchError("expected *v1.Pod but got *v1.Deployment"))
		})
	})
})
