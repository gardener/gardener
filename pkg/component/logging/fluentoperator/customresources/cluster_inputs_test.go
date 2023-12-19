// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package customresources_test

import (
	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2"
	fluentbitv1alpha2input "github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/input"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	. "github.com/gardener/gardener/pkg/component/logging/fluentoperator/customresources"
)

var _ = Describe("Logging", func() {
	Describe("#GetClusterInputs", func() {
		var (
			labels = map[string]string{"some-key": "some-value"}
		)

		It("should return the expected ClusterInput custom resources", func() {
			fluentBitClusterInputs := GetClusterInputs(labels)

			Expect(fluentBitClusterInputs).To(Equal(
				[]*fluentbitv1alpha2.ClusterInput{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "tail-kubernetes",
							Labels: labels,
						},
						Spec: fluentbitv1alpha2.InputSpec{
							Tail: &fluentbitv1alpha2input.Tail{
								Tag:                    "kubernetes.*",
								Path:                   "/var/log/containers/*.log",
								ExcludePath:            "*_garden_fluent-bit-*.log,*_garden_vali-*.log",
								RefreshIntervalSeconds: pointer.Int64(10),
								MemBufLimit:            "30MB",
								SkipLongLines:          pointer.Bool(true),
								DB:                     "/var/fluentbit/flb_kube.db",
								IgnoreOlder:            "30m",
							},
						},
					},
				}))
		})
	})
})
