// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	monitoringutils "github.com/gardener/gardener/pkg/component/monitoring/utils"
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
			Expect(monitoringutils.StandardMetricRelabelConfig("foo", "bar", "baz")).To(HaveExactElements(&monitoringv1.RelabelConfig{
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
