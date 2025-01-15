// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"github.com/gardener/gardener/pkg/component/test"
)

var _ = ginkgo.Describe("PrometheusRules", func() {
	ginkgo.Describe("#CentralPrometheusRules", func() {
		ginkgo.DescribeTable("return the expected objects",
			func(isWorkerless, wantsAlertmanager bool, expectedRuleNames []string) {
				var matchers []any

				for _, ruleName := range expectedRuleNames {
					matchers = append(matchers, PointTo(MatchFields(IgnoreExtras, Fields{
						"TypeMeta":   MatchFields(IgnoreExtras, Fields{"APIVersion": Equal("monitoring.coreos.com/v1"), "Kind": Equal("PrometheusRule")}),
						"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal(ruleName)}),
					})))
				}

				Expect(CentralPrometheusRules(isWorkerless, wantsAlertmanager)).To(HaveExactElements(matchers...))
			},

			ginkgo.Entry("workerless, w/o alertmanager", true, false, []string{"prometheus", "verticalpodautoscaler", "kube-pods", "networking"}),
			ginkgo.Entry("workerless, w/ alertmanager", true, true, []string{"prometheus", "verticalpodautoscaler", "kube-pods", "networking", "alertmanager"}),
			ginkgo.Entry("w/ workers, w/o alertmanager", false, false, []string{"prometheus", "verticalpodautoscaler", "kube-kubelet", "kube-pods", "networking"}),
			ginkgo.Entry("w/ workers, w/ alertmanager", false, true, []string{"prometheus", "verticalpodautoscaler", "kube-kubelet", "kube-pods", "networking", "alertmanager"}),
		)

		ginkgo.It("should run the rules tests", func() {
			test.PrometheusRule(prometheus, "testdata/prometheus.prometheusrule.test.yaml")
			test.PrometheusRule(vpa, "testdata/verticalpodautoscaler.prometheusrule.test.yaml")
			test.PrometheusRule(workerKubeKubelet, "testdata/worker/kube-kubelet.prometheusrule.test.yaml")
			test.PrometheusRule(workerKubePods, "testdata/worker/kube-pods.prometheusrule.test.yaml")
		})
	})
})
