// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cache_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/cache"
)

var _ = Describe("ScrapeConfigs", func() {
	Describe("#CentralScrapeConfigs", func() {
		It("should return the expected objects", func() {
			Expect(cache.CentralScrapeConfigs()).To(HaveExactElements(
				&monitoringv1alpha1.ScrapeConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: "prometheus-cache",
					},
					Spec: monitoringv1alpha1.ScrapeConfigSpec{
						RelabelConfigs: []monitoringv1.RelabelConfig{{
							Action:      "replace",
							Replacement: new("prometheus-cache"),
							TargetLabel: "job",
						}},
						StaticConfigs: []monitoringv1alpha1.StaticConfig{{
							Targets: []monitoringv1alpha1.Target{"localhost:9090"},
						}},
					},
				},
			))
		})
	})
})
