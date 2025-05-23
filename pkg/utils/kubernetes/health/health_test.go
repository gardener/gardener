// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("health", func() {
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
})
