// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package customresources_test

import (
	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	. "github.com/gardener/gardener/pkg/component/logging/fluentoperator/customresources"
)

var _ = Describe("Logging", func() {
	Describe("#GetClusterFluentBitConfig", func() {
		var (
			name                = "fuent-bit"
			fluentBitConfigName = "fluent-bit-config"
			matchLabels         = map[string]string{"some-key": "some-value"}
		)

		It("should return the expected ClusterParser custom resources", func() {
			clusterFluentBitConfig := GetClusterFluentBitConfig(name, matchLabels)

			Expect(clusterFluentBitConfig).To(Equal(
				&fluentbitv1alpha2.ClusterFluentBitConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: fluentBitConfigName,
						Labels: map[string]string{
							"app.kubernetes.io/name": name,
						},
					},
					Spec: fluentbitv1alpha2.FluentBitConfigSpec{
						Service: &fluentbitv1alpha2.Service{
							FlushSeconds: pointer.Int64(30),
							Daemon:       pointer.Bool(false),
							LogLevel:     "error",
							ParsersFile:  "parsers.conf",
							HttpServer:   pointer.Bool(true),
							HttpListen:   "0.0.0.0",
							HttpPort:     pointer.Int32(2020),
						},
						InputSelector: metav1.LabelSelector{
							MatchLabels: matchLabels,
						},
						FilterSelector: metav1.LabelSelector{
							MatchLabels: matchLabels,
						},
						ParserSelector: metav1.LabelSelector{
							MatchLabels: matchLabels,
						},
						OutputSelector: metav1.LabelSelector{
							MatchLabels: matchLabels,
						},
					},
				}))
		})
	})
})
