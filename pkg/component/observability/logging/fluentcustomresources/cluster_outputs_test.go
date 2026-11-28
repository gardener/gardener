// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package fluentcustomresources_test

import (
	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2"
	"github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2/plugins/custom"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/gardener/gardener/pkg/component/observability/logging/fluentcustomresources"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Logging", func() {
	var (
		labels = map[string]string{"some-key": "some-value"}
	)

	Describe("#GetDefaultClusterOutput", func() {
		It("should return the expected DefaultClusterOutput custom resources", func() {
			fluentBitClusterOutputs := GetDefaultClusterOutput(labels, true)

			Expect(fluentBitClusterOutputs).To(Equal(
				&fluentbitv1alpha2.ClusterOutput{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "systemd-default",
						Labels: labels,
					},
					Spec: fluentbitv1alpha2.OutputSpec{
						CustomPlugin: &custom.CustomPlugin{
							Config: `Name gardener
Match                     systemd.*
SeedType                  otlp_grpc
LogLevel                  error
Endpoint                  opentelemetry-collector-collector.garden.svc:4317
Insecure                  true
DQueDir                   /var/fluentbit/dque
DQueName                  systemd
Origin                    systemd
HostnameValue             ${NODE_NAME}
FallbackToTagWhenMetadataIsMissing false`,
						},
					},
				},
			))
		})
		DescribeTable("should return the ClusterOutput with the expected name depending on the feature gate state and whether vali is running",
			func(openTelemetryCollectorEnabled, isValiRunning bool, expectedName *string) {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.OpenTelemetryCollector, openTelemetryCollectorEnabled))

				clusterOutput := GetDefaultClusterOutput(labels, isValiRunning)

				if expectedName == nil {
					Expect(clusterOutput).To(BeNil())
					return
				}
				Expect(clusterOutput.Name).To(Equal(*expectedName))
			},
			Entry("OpenTelemetryCollector feature gate active, vali running", true, true, new("systemd-default")),
			Entry("OpenTelemetryCollector feature gate active, vali not running", true, false, new("systemd-default")),
			Entry("OpenTelemetryCollector feature gate inactive, vali running", false, true, new("journald-default")),
			Entry("OpenTelemetryCollector feature gate inactive, vali not running", false, false, nil),
		)
	})

	Describe("#GetDynamicClusterOutput", func() {
		It("should return the expected DynamicClusterOutput custom resources", func() {
			fluentBitClusterOutputs := GetDynamicClusterOutput(labels, true)

			Expect(fluentBitClusterOutputs).To(Equal(
				&fluentbitv1alpha2.ClusterOutput{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "opentelemetry",
						Labels: labels,
					},
					Spec: fluentbitv1alpha2.OutputSpec{
						CustomPlugin: &custom.CustomPlugin{
							Config: `Name gardener
Match                     kubernetes.*
LogLevel                  info
Retry_Limit               10
SeedType                  otlp_grpc
ShootType                 otlp_grpc
Endpoint                  opentelemetry-collector-collector.garden.svc:4317
Insecure                  true
Timeout                   15m

DynamicHostPath           {"kubernetes": {"namespace_name": "namespace"}}
DynamicHostPrefix         opentelemetry-collector-collector.
DynamicHostSuffix         .svc.cluster.local:4317
DynamicHostRegex          ^shoot-

DQueDir                   /var/fluentbit/dque
DQueName                  dynamic
DQueSync                  normal

DQueBatchProcessorMaxQueueSize    15000
DQueBatchProcessorMaxBatchSize    500
DQueBatchProcessorExportInterval  1s
DQueBatchProcessorExportTimeout   15m
RetryEnabled              true
RetryInitialInterval      1s
RetryMaxInterval          5m
RetryMaxElapsedTime       15m

HostnameValue          ${NODE_NAME}
Origin                 seed
FallbackToTagWhenMetadataIsMissing true
SendLogsToSeedWhenShootIsInHibernatedState false
SendLogsToShootWhenIsInDeletionState false
TagKey                    tag`,
						},
					},
				},
			))
		})

		DescribeTable("should return the ClusterOutput with the expected name depending on the feature gate state and whether vali is running",
			func(openTelemetryCollectorEnabled, isValiRunning bool, expectedName *string) {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.OpenTelemetryCollector, openTelemetryCollectorEnabled))

				clusterOutput := GetDynamicClusterOutput(labels, isValiRunning)

				if expectedName == nil {
					Expect(clusterOutput).To(BeNil())
					return
				}
				Expect(clusterOutput.Name).To(Equal(*expectedName))
			},
			Entry("OpenTelemetryCollector feature gate active, vali running", true, true, new("opentelemetry")),
			Entry("OpenTelemetryCollector feature gate active, vali not running", true, false, new("opentelemetry")),
			Entry("OpenTelemetryCollector feature gate inactive, vali running", false, true, new("gardener-vali")),
			Entry("OpenTelemetryCollector feature gate inactive, vali not running", false, false, nil),
		)
	})

	Describe("#GetStaticClusterOutput", func() {
		It("should return the expected DynamicClusterOutput custom resources", func() {
			fluentBitClusterOutputs := GetStaticClusterOutput(labels, true)

			Expect(fluentBitClusterOutputs).To(Equal(
				&fluentbitv1alpha2.ClusterOutput{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "opentelemetry-static",
						Labels: labels,
					},
					Spec: fluentbitv1alpha2.OutputSpec{
						CustomPlugin: &custom.CustomPlugin{
							Config: `Name gardener
Match                     kubernetes.*
SeedType                  otlp_grpc
LogLevel                  error
Endpoint                  opentelemetry-collector-collector.garden.svc:4317
Insecure                  true
DQueDir                   /var/fluentbit/dque
DQueName                  garden
Origin                    garden
HostnameValue             ${NODE_NAME}
FallbackToTagWhenMetadataIsMissing true`,
						},
					},
				},
			))
		})

		DescribeTable("should return the ClusterOutput with the expected name depending on the feature gate state and whether vali is running",
			func(openTelemetryCollectorEnabled, isValiRunning bool, expectedName *string) {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.OpenTelemetryCollector, openTelemetryCollectorEnabled))

				clusterOutput := GetStaticClusterOutput(labels, isValiRunning)

				if expectedName == nil {
					Expect(clusterOutput).To(BeNil())
					return
				}
				Expect(clusterOutput.Name).To(Equal(*expectedName))
			},
			Entry("OpenTelemetryCollector feature gate active, vali running", true, true, new("opentelemetry-static")),
			Entry("OpenTelemetryCollector feature gate active, vali not running", true, false, new("opentelemetry-static")),
			Entry("OpenTelemetryCollector feature gate inactive, vali running", false, true, new("static-vali")),
			Entry("OpenTelemetryCollector feature gate inactive, vali not running", false, false, nil),
		)
	})
})
