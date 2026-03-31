// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package eventlogger_test

import (
	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2"
	fluentbitv1alpha2filter "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2/plugins/filter"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/gardener/gardener/pkg/component/observability/logging/eventlogger"
	"github.com/gardener/gardener/pkg/features"
	testutils "github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Logging", func() {
	Describe("#CentralLoggingConfiguration", func() {
		BeforeEach(func() {
			DeferCleanup(testutils.WithFeatureGate(features.DefaultFeatureGate, features.OpenTelemetryCollector, false))
		})
		It("should return the expected logging parser and filter", func() {
			loggingConfig, err := CentralLoggingConfiguration()

			Expect(err).NotTo(HaveOccurred())

			if features.DefaultFeatureGate.Enabled(features.OpenTelemetryCollector) {
				Expect(loggingConfig.Filters).To(Equal(
					[]*fluentbitv1alpha2.ClusterFilter{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:   "event-logger",
								Labels: map[string]string{"fluentbit.gardener/type": "seed"},
							},
							Spec: fluentbitv1alpha2.FilterSpec{
								Match: "kubernetes.*event-logger*event-logger*",
								FilterItems: []fluentbitv1alpha2.FilterItem{
									{
										RecordModifier: &fluentbitv1alpha2filter.RecordModifier{
											Records: []string{"job event-logging"},
										},
									},
								},
							},
						},
					}))
			} else {
				Expect(loggingConfig.Filters).To(Equal(
					[]*fluentbitv1alpha2.ClusterFilter{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:   "event-logger",
								Labels: map[string]string{"fluentbit.gardener/type": "seed"},
							},
							Spec: fluentbitv1alpha2.FilterSpec{
								Match: "kubernetes.*event-logger*event-logger*",
								FilterItems: []fluentbitv1alpha2.FilterItem{
									{
										RecordModifier: &fluentbitv1alpha2filter.RecordModifier{
											Records: []string{"job event-logging"},
										},
									},
									{
										Nest: &fluentbitv1alpha2filter.Nest{
											Operation:   "lift",
											NestedUnder: "log",
										},
									},
								},
							},
						},
					}))
			}
			Expect(loggingConfig.Inputs).To(BeNil())
			Expect(loggingConfig.Parsers).To(BeNil())
		})
	})
})
