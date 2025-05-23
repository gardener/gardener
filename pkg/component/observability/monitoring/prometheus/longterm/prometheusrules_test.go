// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package longterm

import (
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = ginkgo.Describe("PrometheusRules", func() {
	ginkgo.Describe("#CentralPrometheusRules", func() {
		ginkgo.It("should return the expected objects", func() {
			Expect(CentralPrometheusRules()).To(HaveExactElements(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"TypeMeta":   MatchFields(IgnoreExtras, Fields{"APIVersion": Equal("monitoring.coreos.com/v1"), "Kind": Equal("PrometheusRule")}),
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("recording")}),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"TypeMeta":   MatchFields(IgnoreExtras, Fields{"APIVersion": Equal("monitoring.coreos.com/v1"), "Kind": Equal("PrometheusRule")}),
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("sla")}),
				})),
			))
		})
	})
})
