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
	// This endpoint will be hard to find in the OpenTelemetry Collector documentation. Since the OTLP exporter
	// works via gRPC, the structure of the path is defined by the gRPC spec and it's not explicitly documented in the OpenTelemetry docs.
	// The meaning of this URL in the gRPC world is:
	// - "opentelemetry.proto.collector.logs.v1.LogsService" is the service
	// - "Export" is the method of that service
	PushEndpoint = "/opentelemetry.proto.collector.logs.v1.LogsService/Export"
	// PushPort is the port that the OTLP receiver listens on in the OpenTelemetry Collector deployment.
	PushPort = 4317
	// KubeRBACProxyPort is the port that the KubeRBACProxy listens on in the OpenTelemetry Collector deployment.
	KubeRBACProxyPort = 8080
)
