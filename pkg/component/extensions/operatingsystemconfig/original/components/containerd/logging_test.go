// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package containerd_test

import (
	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2"
	fluentbitv1alpha2filter "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2/plugins/filter"
	fluentbitv1alpha2input "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2/plugins/input"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/containerd"
)

var _ = Describe("Logging", func() {
	Describe("#CentralLoggingConfiguration", func() {
		It("should return the expected logging inputs and filters", func() {
			loggingConfig, err := CentralLoggingConfiguration()

			Expect(err).NotTo(HaveOccurred())
			Expect(loggingConfig.Inputs).To(Equal(
				[]*fluentbitv1alpha2.ClusterInput{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "journald-containerd",
							Labels: map[string]string{"fluentbit.gardener/type": "seed"},
						},
						Spec: fluentbitv1alpha2.InputSpec{
							Systemd: &fluentbitv1alpha2input.Systemd{
								Tag:           "journald.containerd",
								ReadFromTail:  "on",
								SystemdFilter: []string{"_SYSTEMD_UNIT=containerd.service"},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "journald-containerd-monitor",
							Labels: map[string]string{"fluentbit.gardener/type": "seed"},
						},
						Spec: fluentbitv1alpha2.InputSpec{
							Systemd: &fluentbitv1alpha2input.Systemd{
								Tag:           "journald.containerd-monitor",
								ReadFromTail:  "on",
								SystemdFilter: []string{"_SYSTEMD_UNIT=containerd-monitor.service"},
							},
						},
					},
				}))

			Expect(loggingConfig.Filters).To(Equal(
				[]*fluentbitv1alpha2.ClusterFilter{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "journald-containerd",
							Labels: map[string]string{"fluentbit.gardener/type": "seed"},
						},
						Spec: fluentbitv1alpha2.FilterSpec{
							Match: "journald.containerd",
							FilterItems: []fluentbitv1alpha2.FilterItem{
								{
									RecordModifier: &fluentbitv1alpha2filter.RecordModifier{
										Records: []string{"hostname ${NODE_NAME}", "unit containerd"},
									},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "journald-containerd-monitor",
							Labels: map[string]string{"fluentbit.gardener/type": "seed"},
						},
						Spec: fluentbitv1alpha2.FilterSpec{
							Match: "journald.containerd-monitor",
							FilterItems: []fluentbitv1alpha2.FilterItem{
								{
									RecordModifier: &fluentbitv1alpha2filter.RecordModifier{
										Records: []string{"hostname ${NODE_NAME}", "unit containerd-monitor"},
									},
								},
							},
						},
					},
				}))
			Expect(loggingConfig.Parsers).To(BeNil())
		})
	})
})
