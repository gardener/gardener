// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package prometheus_test

import (
	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2"
	fluentbitv1alpha2filter "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2/plugins/filter"
	fluentbitv1alpha2parser "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2/plugins/parser"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus"
)

var _ = Describe("Logging", func() {
	Describe("#CentralLoggingConfiguration", func() {
		It("should return the expected logging parser and filter", func() {
			loggingConfig, err := CentralLoggingConfiguration()

			Expect(err).NotTo(HaveOccurred())
			Expect(loggingConfig.Filters).To(Equal(
				[]*fluentbitv1alpha2.ClusterFilter{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "prometheus--prometheus",
							Labels: map[string]string{"fluentbit.gardener/type": "seed"},
						},
						Spec: fluentbitv1alpha2.FilterSpec{
							Match: "kubernetes.*prometheus*prometheus*",
							FilterItems: []fluentbitv1alpha2.FilterItem{
								{
									Parser: &fluentbitv1alpha2filter.Parser{
										KeyName:     "log",
										Parser:      "prometheus-parser",
										ReserveData: ptr.To(true),
									},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "prometheus--blackbox-exporter",
							Labels: map[string]string{"fluentbit.gardener/type": "seed"},
						},
						Spec: fluentbitv1alpha2.FilterSpec{
							Match: "kubernetes.*prometheus*blackbox-exporter*",
							FilterItems: []fluentbitv1alpha2.FilterItem{
								{
									Parser: &fluentbitv1alpha2filter.Parser{
										KeyName:     "log",
										Parser:      "prometheus-parser",
										ReserveData: ptr.To(true),
									},
								},
							},
						},
					},
				}))

			Expect(loggingConfig.Parsers).To(Equal(
				[]*fluentbitv1alpha2.ClusterParser{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "prometheus-parser",
							Labels: map[string]string{"fluentbit.gardener/type": "seed"},
						},
						Spec: fluentbitv1alpha2.ParserSpec{
							Regex: &fluentbitv1alpha2parser.Regex{
								Regex:      "^ts=(?<time>\\d{4}-\\d{2}-\\d{2}[Tt]{1}\\d{2}:\\d{2}:\\d{2}\\.\\d+\\S+)\\s+caller=(?<source>.+?)\\s+level=(?<severity>\\w+)\\s+(?<log>.*)$",
								TimeKey:    "time",
								TimeFormat: "%Y-%m-%dT%H:%M:%S.%L%z",
							},
						},
					},
				}))
			Expect(loggingConfig.Inputs).To(BeNil())
		})
	})
})
