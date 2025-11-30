// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package istio_test

import (
	"regexp"

	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2"
	fluentbitv1alpha2filter "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2/plugins/filter"
	fluentbitv1alpha2parser "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2/plugins/parser"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/component/networking/istio"
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
							Name:   "istio-ingressgateway",
							Labels: map[string]string{"fluentbit.gardener/type": "seed"},
						},
						Spec: fluentbitv1alpha2.FilterSpec{
							Match: "kubernetes.*istio-ingressgateway*istio-proxy*",
							FilterItems: []fluentbitv1alpha2.FilterItem{
								{
									Parser: &fluentbitv1alpha2filter.Parser{
										KeyName:     "log",
										Parser:      "istio-proxy-parser",
										ReserveData: ptr.To(true),
										PreserveKey: ptr.To(true),
									},
								},
								{
									Lua: &fluentbitv1alpha2filter.Lua{
										Call: "add_kubernetes_namespace_name_to_record",
										Script: corev1.ConfigMapKeySelector{
											Key: "add_kubernetes_namespace_name_to_record.lua",
											LocalObjectReference: corev1.LocalObjectReference{
												Name: v1beta1constants.ConfigMapNameFluentBitLua,
											},
										},
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
							Name:   "istio-proxy-parser",
							Labels: map[string]string{"fluentbit.gardener/type": "seed"},
						},
						Spec: fluentbitv1alpha2.ParserSpec{
							Regex: &fluentbitv1alpha2parser.Regex{
								Regex: `^.*\.(?<namespace_name>shoot--[a-zA-Z0-9_-]+)\.svc\.cluster\.local.*$`,
							},
						},
					},
				}))
			Expect(loggingConfig.Inputs).To(BeNil())
		})
		It("should parse istio-proxy log entries", func() {
			loggingConfig, err := CentralLoggingConfiguration()
			Expect(err).NotTo(HaveOccurred())

			re := regexp.MustCompile(loggingConfig.Parsers[0].Spec.Regex.Regex)
			proxyLogLine := `[2025-11-26T07:47:54.896Z] "- - -" 0 - - - "-" 5470 16766 90055 - "-" "-" "-" "-" "10.193.3.153:443" outbound|443||kube-apiserver.shoot--projectname--shootname.svc.cluster.local 10.193.0.238:52412 10.193.0.238:9443 10.193.8.156:35300 api.shootname.projectname.internal.gardener.our.test.domain -`
			Expect(re.MatchString(proxyLogLine)).To(BeTrue())

			matches := re.FindStringSubmatch(proxyLogLine)
			idx := re.SubexpIndex("namespace_name")
			Expect(matches[idx]).To(Equal("shoot--projectname--shootname"))
		})
	})
})
