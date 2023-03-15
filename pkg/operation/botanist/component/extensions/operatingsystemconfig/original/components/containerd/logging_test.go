// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package containerd_test

import (
	fluentbitv1alpha2 "github.com/fluent/fluent-operator/apis/fluentbit/v1alpha2"
	fluentbitv1alpha2filter "github.com/fluent/fluent-operator/apis/fluentbit/v1alpha2/plugins/filter"
	fluentbitv1alpha2input "github.com/fluent/fluent-operator/apis/fluentbit/v1alpha2/plugins/input"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/containerd"
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
			Expect(loggingConfig.PodPrefixes).To(BeEmpty())
			Expect(loggingConfig.UserExposed).To(BeFalse())

		})
	})
})
