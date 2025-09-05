// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package constants

const (
	// OpenTelemetryCollectorResourceName is the name of the OpenTelemetry Collector resource.
	OpenTelemetryCollectorResourceName = "opentelemetry-collector"
	// DeploymentName is the name that the OpenTelemetry Operator will for the Collector deployment.
	// Note: Currently, the OpenTelemetry Operator hardcodes the deployment name to be the same as the resource name with a '-collector' suffix.
	DeploymentName = OpenTelemetryCollectorResourceName + "-collector"
	// ServiceName is the name the OpenTelemetry Operator will use for the Collector service.
	// Note: Currently, the OpenTelemetry Operator hardcodes the service name to be the same as the resource name with a '-collector' suffix.
	ServiceName = OpenTelemetryCollectorResourceName + "-collector"
	// ServiceAccountName is the name of the ServiceAccount used by the OpenTelemetry Collector.
	ServiceAccountName = OpenTelemetryCollectorResourceName
	// PushEndpoint is the endpoint where the OpenTelemetry Collector receives logs from log shippers.
	PushEndpoint = "/opentelemetry.proto.collector.logs.v1.LogsService/Export"
	// PushPort is the port that the OTLP receiver listens on in the OpenTelemetry Collector deployment.
	PushPort = 4317
	// KubeRBACProxyPort is the port that the KubeRBACProxy listens on in the OpenTelemetry Collector deployment.
	KubeRBACProxyPort = 8080
)
