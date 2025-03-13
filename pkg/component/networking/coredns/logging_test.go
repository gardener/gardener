// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package coredns_test

import (
	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2"
	fluentbitv1alpha2filter "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2/plugins/filter"
	fluentbitv1alpha2parser "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2/plugins/parser"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/component/networking/coredns"
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
							Name:   "coredns",
							Labels: map[string]string{"fluentbit.gardener/type": "seed"},
						},
						Spec: fluentbitv1alpha2.FilterSpec{
							Match: "kubernetes.*coredns*coredns*",
							FilterItems: []fluentbitv1alpha2.FilterItem{
								{
									Parser: &fluentbitv1alpha2filter.Parser{
										KeyName:     "log",
										Parser:      "coredns-parser1,coredns-parser2",
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
							Name:   "coredns-parser1",
							Labels: map[string]string{"fluentbit.gardener/type": "seed"},
						},
						Spec: fluentbitv1alpha2.ParserSpec{
							Regex: &fluentbitv1alpha2parser.Regex{
								Regex:      "^(?<time>\\d{4}-\\d{2}-\\d{2}[Tt].*[zZ])\\s+\\[(?<severity>\\w*[^\\]])\\]\\s+(?<log>.*)",
								TimeKey:    "time",
								TimeFormat: "%Y-%m-%dT%H:%M:%S.%L",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "coredns-parser2",
							Labels: map[string]string{"fluentbit.gardener/type": "seed"},
						},
						Spec: fluentbitv1alpha2.ParserSpec{
							Regex: &fluentbitv1alpha2parser.Regex{
								Regex:      "^(?<severity>\\w)(?<time>\\d{4} [^\\s]*)\\s+(?<pid>\\d+)\\s+(?<source>[^ \\]]+)\\] (?<log>.*)$",
								TimeKey:    "time",
								TimeFormat: "%m%d %H:%M:%S.%L",
							},
						},
					},
				}))
			Expect(loggingConfig.Inputs).To(BeNil())
		})
	})
})
