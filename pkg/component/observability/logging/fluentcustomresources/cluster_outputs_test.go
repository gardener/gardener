// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
)

var _ = Describe("Logging", func() {
	Describe("#GetDefaultClusterOutputs", func() {
		var (
			labels = map[string]string{"some-key": "some-value"}
		)

		It("should return the expected DefaultClusterOutput custom resources", func() {
			fluentBitClusterOutputs := GetDefaultClusterOutput(labels)

			Expect(fluentBitClusterOutputs).To(Equal(
				&fluentbitv1alpha2.ClusterOutput{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "systemd",
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
	})

	Describe("#GetDynamicClusterOutput", func() {
		var (
			labels = map[string]string{"some-key": "some-value"}
		)

		It("should return the expected DynamicClusterOutput custom resources", func() {
			fluentBitClusterOutputs := GetDynamicClusterOutput(labels)

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
LogLevel                  error
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
TagKey                    tag`,
						},
					},
				},
			))
		})
	})

	Describe("#GetStaticClusterOutput", func() {
		var (
			labels = map[string]string{"some-key": "some-value"}
		)

		It("should return the expected DynamicClusterOutput custom resources", func() {
			fluentBitClusterOutputs := GetStaticClusterOutput(labels)

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
FallbackToTagWhenMetadataIsMissing false`,
						},
					},
				},
			))
		})
	})
})
