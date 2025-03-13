// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seedserver_test

import (
	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2"
	fluentbitv1alpha2filter "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2/plugins/filter"
	fluentbitv1alpha2parser "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2/plugins/parser"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/component/networking/vpn/seedserver"
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
							Name:   "vpn-seed-server--vpn-seed-server",
							Labels: map[string]string{"fluentbit.gardener/type": "seed"},
						},
						Spec: fluentbitv1alpha2.FilterSpec{
							Match: "kubernetes.*vpn-seed-server*vpn-seed-server*",
							FilterItems: []fluentbitv1alpha2.FilterItem{
								{
									Parser: &fluentbitv1alpha2filter.Parser{
										KeyName:     "log",
										Parser:      "vpn-seed-server-parser",
										ReserveData: ptr.To(true),
									},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "vpn-seed-server--envoy-proxy",
							Labels: map[string]string{"fluentbit.gardener/type": "seed"},
						},
						Spec: fluentbitv1alpha2.FilterSpec{
							Match: "kubernetes.*vpn-seed-server*envoy-proxy*",
							FilterItems: []fluentbitv1alpha2.FilterItem{
								{
									Parser: &fluentbitv1alpha2filter.Parser{
										KeyName:     "log",
										Parser:      "envoy-proxy-parser",
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
							Name:   "vpn-seed-server-parser",
							Labels: map[string]string{"fluentbit.gardener/type": "seed"},
						},
						Spec: fluentbitv1alpha2.ParserSpec{
							Regex: &fluentbitv1alpha2parser.Regex{
								Regex:      "^(?<time>\\d{4}-\\d{2}-\\d{2}\\s+[^\\s]+)\\s+(?<log>.*)$",
								TimeKey:    "time",
								TimeFormat: "%Y-%m-%d %H:%M:%S",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "envoy-proxy-parser",
							Labels: map[string]string{"fluentbit.gardener/type": "seed"},
						},
						Spec: fluentbitv1alpha2.ParserSpec{
							Regex: &fluentbitv1alpha2parser.Regex{
								Regex:      "^\\[(?<time>\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}\\.\\d+\\S+?)]\\s+(?<log>.*)$",
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
