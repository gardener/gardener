// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"github.com/gardener/gardener/pkg/component/test"
)

var _ = ginkgo.Describe("PrometheusRules", func() {
	ginkgo.Describe("#CentralPrometheusRules", func() {
		ginkgo.DescribeTable("should return the expected objects", func(isGardenerDiscoveryServerEnabled bool) {
			config := CentralPrometheusRules(isGardenerDiscoveryServerEnabled)
			Expect(config).To(HaveExactElements(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"TypeMeta":   MatchFields(IgnoreExtras, Fields{"APIVersion": Equal("monitoring.coreos.com/v1"), "Kind": Equal("PrometheusRule")}),
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("auditlog")}),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"TypeMeta":   MatchFields(IgnoreExtras, Fields{"APIVersion": Equal("monitoring.coreos.com/v1"), "Kind": Equal("PrometheusRule")}),
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("etcd")}),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"TypeMeta":   MatchFields(IgnoreExtras, Fields{"APIVersion": Equal("monitoring.coreos.com/v1"), "Kind": Equal("PrometheusRule")}),
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("garden")}),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"TypeMeta":   MatchFields(IgnoreExtras, Fields{"APIVersion": Equal("monitoring.coreos.com/v1"), "Kind": Equal("PrometheusRule")}),
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("metering-meta")}),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"TypeMeta":   MatchFields(IgnoreExtras, Fields{"APIVersion": Equal("monitoring.coreos.com/v1"), "Kind": Equal("PrometheusRule")}),
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("recording")}),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"TypeMeta":   MatchFields(IgnoreExtras, Fields{"APIVersion": Equal("monitoring.coreos.com/v1"), "Kind": Equal("PrometheusRule")}),
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("seed")}),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"TypeMeta":   MatchFields(IgnoreExtras, Fields{"APIVersion": Equal("monitoring.coreos.com/v1"), "Kind": Equal("PrometheusRule")}),
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("shoot")}),
				})),
			))

			testGardenerDiscoverServerAlert(config, isGardenerDiscoveryServerEnabled)

			test.PrometheusRule(metering, "testdata/metering-meta.prometheusrule.test.yaml")
			test.PrometheusRule(seed, "testdata/seed.prometheusrule.test.yaml")
			test.PrometheusRule(shoot, "testdata/shoot.prometheusrule.test.yaml")
			test.PrometheusRule(etcd, "testdata/etcd.prometheusrule.test.yaml")
		},
			ginkgo.Entry("when gardener discovery server is enabled", true),
			ginkgo.Entry("when gardener discovery server is disabled", false),
		)
	})
})

func testGardenerDiscoverServerAlert(config []*monitoringv1.PrometheusRule, isGardenerDiscoveryServerEnabled bool) {
	var gardenConfig *monitoringv1.PrometheusRule
	for _, c := range config {
		if c.Name == "garden" {
			gardenConfig = c
		}
	}
	Expect(gardenConfig).ToNot(BeNil())

	var gardenGroup *monitoringv1.RuleGroup
	for _, g := range gardenConfig.Spec.Groups {
		if g.Name == "garden" {
			gardenGroup = &g
		}
	}
	Expect(gardenGroup).ToNot(BeNil())

	var discoveryServerAlert *monitoringv1.Rule
	for _, r := range gardenGroup.Rules {
		if r.Alert == "DiscoveryServerDown" {
			discoveryServerAlert = &r
		}
	}

	if isGardenerDiscoveryServerEnabled {
		Expect(discoveryServerAlert).ToNot(BeNil())
	} else {
		Expect(discoveryServerAlert).To(BeNil())
	}
}
