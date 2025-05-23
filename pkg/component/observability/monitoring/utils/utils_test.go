// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
)

var _ = Describe("Utils", func() {
	Describe("#ConfigObjectMeta", func() {
		It("should return the expected object meta", func() {
			Expect(monitoringutils.ConfigObjectMeta("foo", "bar", "baz")).To(Equal(metav1.ObjectMeta{
				Name:      "baz-foo",
				Namespace: "bar",
				Labels:    map[string]string{"prometheus": "baz"},
			}))
		})
	})

	Describe("#StandardMetricRelabelConfig", func() {
		It("should return the expected relabel configs", func() {
			Expect(monitoringutils.StandardMetricRelabelConfig("foo", "bar", "baz")).To(HaveExactElements(monitoringv1.RelabelConfig{
				SourceLabels: []monitoringv1.LabelName{"__name__"},
				Action:       "keep",
				Regex:        `^(foo|bar|baz)$`,
			}))
		})
	})

	Describe("#Labels", func() {
		It("should return the expected labels", func() {
			Expect(monitoringutils.Labels("foo")).To(Equal(map[string]string{"prometheus": "foo"}))
		})
	})
})
