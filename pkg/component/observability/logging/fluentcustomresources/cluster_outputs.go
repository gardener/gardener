// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fluentcustomresources

import (
	"slices"
	"strings"

	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2"
	"github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2/plugins/custom"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/features"
)

const (
	// Output names
	outputNameJournald      = "journald"
	outputNameSystemd       = "systemd"
	outputNameVali          = "gardener-vali"
	outputNameOpenTelemetry = "opentelemetry"
	outputNameStaticVali    = "static-vali"
	outputNameStaticOTel    = "opentelemetry-static"

	// Plugin names
	pluginNameVali     = "gardenervali"
	pluginNameGardener = "gardener"

	// Endpoints
	endpointOTelCollector = "opentelemetry-collector-collector.garden.svc:4317"

	// Match patterns
	matchJournald   = "journald.*"
	matchSystemd    = "systemd.*"
	matchKubernetes = "kubernetes.*"

	// Origins
	originSeedJournald = "seed-journald"
	originSystemd      = "systemd"
	originSeed         = "seed"
	originGarden       = "garden"

	// DQue settings
	dqueDir         = "/var/fluentbit/dque"
	dqueNameSystemd = "systemd"
	dqueNameDynamic = "dynamic"
	dqueNameGarden  = "garden"
	dqueSync        = "normal"

	// Buffer settings for Vali
	bufferDirBase         = "/fluent-bit/buffers"
	bufferDirSeedDynamic  = bufferDirBase + "/seed"
	bufferDirGarden       = bufferDirBase + "/garden"
	queueNameSeedJournald = "seed-journald"
	queueNameSeedDynamic  = "seed-dynamic"
	queueNameStaticGarden = "gardener-operator-static"

	// TODO(nickytd): remove once vali is completely removed
	commonSettings = `LogLevel info
Url http://logging.garden.svc:3100/vali/api/v1/push
BatchWait 5s
BatchSize 2097152
MaxRetries 5
Timeout 10s
MinBackoff 10s
MaxBackoff 300s
LineFormat json
SortByTimestamp true
DropSingleKey false
AutoKubernetesLabels false
HostnameKeyValue nodename ${NODE_NAME}
Buffer true
BufferType dque
QueueSegmentSize 10000
QueueSync normal
NumberOfBatchIDs 5
`
	// TODO(nickytd): remove once vali is completely removed
	staticDynamicCommonSettings = `RemoveKeys kubernetes,stream,time,tag,gardenuser,job
LabelMapPath {"kubernetes": {"container_name":"container_name","container_id":"container_id","namespace_name":"namespace_name","pod_name":"pod_name"},"severity": "severity","job": "job"}
FallbackToTagWhenMetadataIsMissing true
TagKey tag
DropLogEntryWithoutK8sMetadata true
`
)

// GetDefaultClusterOutput returns the default ClusterOutput used by the Fluent Operator.
func GetDefaultClusterOutput(labels map[string]string) *fluentbitv1alpha2.ClusterOutput {
	if slices.ContainsFunc(features.DefaultFeatureGate.KnownFeatures(), func(s string) bool {
		return strings.HasPrefix(s, string(features.OpenTelemetryCollector)+"=")
	}) && features.DefaultFeatureGate.Enabled(features.OpenTelemetryCollector) {
		return &fluentbitv1alpha2.ClusterOutput{
			ObjectMeta: metav1.ObjectMeta{
				Name:   outputNameSystemd,
				Labels: labels,
			},
			Spec: fluentbitv1alpha2.OutputSpec{
				CustomPlugin: &custom.CustomPlugin{
					Config: `Name ` + pluginNameGardener + `
Match                     ` + matchSystemd + `
SeedType                  otlp_grpc
LogLevel                  error
Endpoint                  ` + endpointOTelCollector + `
Insecure                  true
DQueDir                   ` + dqueDir + `
DQueName                  ` + dqueNameSystemd + `
Origin                    ` + originSystemd + `
HostnameValue             ${NODE_NAME}
FallbackToTagWhenMetadataIsMissing false`,
				},
			},
		}
	}

	return &fluentbitv1alpha2.ClusterOutput{
		ObjectMeta: metav1.ObjectMeta{
			Name:   outputNameJournald,
			Labels: labels,
		},
		Spec: fluentbitv1alpha2.OutputSpec{
			CustomPlugin: &custom.CustomPlugin{
				Config: `Name ` + pluginNameVali + `
Match ` + matchJournald + `
Labels {origin="` + originSeedJournald + `"}
RemoveKeys kubernetes,stream,hostname,unit
LabelMapPath {"hostname":"host_name","unit":"systemd_component"}
QueueDir ` + bufferDirBase + `
QueueName ` + queueNameSeedJournald + `
` + commonSettings,
			},
		},
	}
}

// GetDynamicClusterOutput returns the dynamic fluent-bit-to-vali output used by the Fluent Operator.
func GetDynamicClusterOutput(labels map[string]string) *fluentbitv1alpha2.ClusterOutput {
	if features.DefaultFeatureGate.Enabled(features.OpenTelemetryCollector) {
		return &fluentbitv1alpha2.ClusterOutput{
			ObjectMeta: metav1.ObjectMeta{
				Name:   outputNameOpenTelemetry,
				Labels: labels,
			},
			Spec: fluentbitv1alpha2.OutputSpec{
				CustomPlugin: &custom.CustomPlugin{
					Config: `Name ` + pluginNameGardener + `
Match                     ` + matchKubernetes + `
LogLevel                  error
Retry_Limit               10
SeedType                  otlp_grpc
ShootType                 otlp_grpc
Endpoint                  ` + endpointOTelCollector + `
Insecure                  true
Timeout                   15m

DynamicHostPath           {"kubernetes": {"namespace_name": "namespace"}}
DynamicHostPrefix         opentelemetry-collector-collector.
DynamicHostSuffix         .svc.cluster.local:4317
DynamicHostRegex          ^shoot-

DQueDir                   ` + dqueDir + `
DQueName                  ` + dqueNameDynamic + `
DQueSync                  ` + dqueSync + `

DQueBatchProcessorMaxQueueSize    15000
DQueBatchProcessorMaxBatchSize    500
DQueBatchProcessorExportInterval  1s
DQueBatchProcessorExportTimeout   15m
RetryEnabled              true
RetryInitialInterval      1s
RetryMaxInterval          5m
RetryMaxElapsedTime       15m

HostnameValue          ${NODE_NAME}
Origin                 ` + originSeed + `
FallbackToTagWhenMetadataIsMissing true
SendLogsToSeedWhenShootIsInHibernatedState false
SendLogsToShootWhenIsInDeletionState false
TagKey                    tag`,
				},
			},
		}
	}

	return &fluentbitv1alpha2.ClusterOutput{
		ObjectMeta: metav1.ObjectMeta{
			Name:   outputNameVali,
			Labels: labels,
		},
		Spec: fluentbitv1alpha2.OutputSpec{
			CustomPlugin: &custom.CustomPlugin{
				Config: `Name ` + pluginNameVali + `
Match ` + matchKubernetes + `
Labels {origin="` + originSeed + `"}
DropSingleKey false
LabelSelector gardener.cloud/role:shoot
DynamicHostPath {"kubernetes": {"namespace_name": "namespace"}}
DynamicHostPrefix http://logging.
DynamicHostSuffix .svc:3100/vali/api/v1/push
DynamicHostRegex ^shoot-
QueueDir ` + bufferDirSeedDynamic + `
QueueName ` + queueNameSeedDynamic + `
SendDeletedClustersLogsToDefaultClient true
CleanExpiredClientsPeriod 1h
ControllerSyncTimeout 120s
PreservedLabels origin,namespace_name,pod_name
` + commonSettings + staticDynamicCommonSettings,
			},
		},
	}
}

// GetStaticClusterOutput returns the static fluent-bit-to-vali output used by the Fluent Operator.
// This output is present on garden runtime cluster.
func GetStaticClusterOutput(labels map[string]string) *fluentbitv1alpha2.ClusterOutput {
	if slices.ContainsFunc(features.DefaultFeatureGate.KnownFeatures(), func(s string) bool {
		return strings.HasPrefix(s, string(features.OpenTelemetryCollector)+"=")
	}) && features.DefaultFeatureGate.Enabled(features.OpenTelemetryCollector) {
		return &fluentbitv1alpha2.ClusterOutput{
			ObjectMeta: metav1.ObjectMeta{
				Name:   outputNameStaticOTel,
				Labels: labels,
			},
			Spec: fluentbitv1alpha2.OutputSpec{
				CustomPlugin: &custom.CustomPlugin{
					Config: `Name ` + pluginNameGardener + `
Match                     ` + matchKubernetes + `
SeedType                  otlp_grpc
LogLevel                  error
Endpoint                  ` + endpointOTelCollector + `
Insecure                  true
DQueDir                   ` + dqueDir + `
DQueName                  ` + dqueNameGarden + `
Origin                    ` + originGarden + `
HostnameValue             ${NODE_NAME}
FallbackToTagWhenMetadataIsMissing false`,
				},
			},
		}
	}

	return &fluentbitv1alpha2.ClusterOutput{
		ObjectMeta: metav1.ObjectMeta{
			Name:   outputNameStaticVali,
			Labels: labels,
		},
		Spec: fluentbitv1alpha2.OutputSpec{
			CustomPlugin: &custom.CustomPlugin{
				Config: `Name ` + pluginNameVali + `
Match ` + matchKubernetes + `
Labels {origin="` + originGarden + `"}
QueueDir ` + bufferDirGarden + `
QueueName ` + queueNameStaticGarden + `
` + commonSettings + staticDynamicCommonSettings,
			},
		},
	}
}
