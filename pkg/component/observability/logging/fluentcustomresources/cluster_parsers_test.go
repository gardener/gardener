// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fluentcustomresources_test

import (
	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2"
	fluentbitv1alpha2parser "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2/plugins/parser"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/component/observability/logging/fluentcustomresources"
)

var _ = Describe("Logging", func() {
	Describe("#GetClusterParsers", func() {
		var (
			labels = map[string]string{"some-key": "some-value"}
		)

		It("should return the expected ClusterParser custom resources", func() {
			fluentBitClusterParsers := GetClusterParsers(labels)

			Expect(fluentBitClusterParsers).To(Equal(
				[]*fluentbitv1alpha2.ClusterParser{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "containerd-parser",
							Labels: labels,
						},
						Spec: fluentbitv1alpha2.ParserSpec{
							Regex: &fluentbitv1alpha2parser.Regex{
								Regex:      "^(?<time>[^ ]+) (stdout|stderr) ([^ ]*) (?<log>.*)$",
								TimeKey:    "time",
								TimeFormat: "%Y-%m-%dT%H:%M:%S.%L%z",
								TimeKeep:   ptr.To(true),
							},
							Decoders: []fluentbitv1alpha2.Decorder{ // spellchecker:disable-line
								{
									DecodeFieldAs: "json log",
								},
							},
						},
					},
				}))
		})
	})
})
