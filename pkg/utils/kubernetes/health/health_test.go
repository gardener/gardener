// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclock "k8s.io/utils/clock/testing"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("health", func() {
	var (
		fakeClock = testclock.NewFakeClock(time.Now())
	)

	Describe("ObjectHasAnnotationWithValue", func() {
		var (
			healthFunc health.Func
			key, value string
		)

		BeforeEach(func() {
			key = "foo"
			value = "bar"
			healthFunc = health.ObjectHasAnnotationWithValue(key, value)
		})

		It("should fail if object does not have the annotation", func() {
			Expect(healthFunc(&extensionsv1alpha1.Infrastructure{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"other": "bla"},
				},
			})).NotTo(Succeed())
		})
		It("should fail if object's annotation have a different value", func() {
			Expect(healthFunc(&extensionsv1alpha1.Infrastructure{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{key: "nope"},
				},
			})).NotTo(Succeed())
		})
		It("should succeed if object's annotation has the expected value", func() {
			Expect(healthFunc(&extensionsv1alpha1.Infrastructure{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{key: value},
				},
			})).To(Succeed())
		})
	})

	DescribeTable("#IsSkippedUntil", func(annotations map[string]string, expected bool) {
		DeferCleanup(test.WithVar(&health.Clock, fakeClock))

		skipped := health.IsSkippedUntil(&metav1.ObjectMeta{
			Annotations: annotations,
		})

		Expect(skipped).To(Equal(expected))
	},
		Entry("no annotation: not skipped", nil, false),
		Entry("future timestamp: skipped",
			map[string]string{v1beta1constants.AnnotationCareSkipHealthChecksUntil: fakeClock.Now().Add(1 * time.Hour).Format(time.RFC3339)}, true),
		Entry("past timestamp: not skipped",
			map[string]string{v1beta1constants.AnnotationCareSkipHealthChecksUntil: fakeClock.Now().Add(-1 * time.Hour).Format(time.RFC3339)}, false),
		Entry("invalid RFC3339: not skipped",
			map[string]string{v1beta1constants.AnnotationCareSkipHealthChecksUntil: "not-a-timestamp"}, false),
	)
})
