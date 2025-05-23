// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cache_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/cache"
)

var _ = Describe("PrometheusRules", func() {
	Describe("#CentralPrometheusRules", func() {
		It("should return the expected objects", func() {
			Expect(cache.CentralPrometheusRules()).To(HaveExactElements(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"TypeMeta":   MatchFields(IgnoreExtras, Fields{"APIVersion": Equal("monitoring.coreos.com/v1"), "Kind": Equal("PrometheusRule")}),
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("metering")}),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"TypeMeta":   MatchFields(IgnoreExtras, Fields{"APIVersion": Equal("monitoring.coreos.com/v1"), "Kind": Equal("PrometheusRule")}),
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("metering-stateful")}),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"TypeMeta":   MatchFields(IgnoreExtras, Fields{"APIVersion": Equal("monitoring.coreos.com/v1"), "Kind": Equal("PrometheusRule")}),
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("recording-rules")}),
				})),
			))
		})
	})
})
