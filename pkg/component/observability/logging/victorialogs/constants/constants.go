// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package constants

const (
	// VictoriaLogsHttpPort is the HTTP port exposed by VictoriaLogs.
	VictoriaLogsHttpPort = 9428
	// VictoriaLogsPort is the HTTPS port exposed by VictoriaLogs.
	VictoriaLogsPort = 9429
	// ServiceName is the name of the logging service.
	ServiceName = "logging-vl"
	// PushEndpoint is the endpoint used by VictoriaLogs to receive logs.
	PushEndpoint = "/insert/opentelemetry/v1/logs"
	// ManagedResourceNameRuntime is the name of the managed resource which deploys VictoriaLogs.
	ManagedResourceNameRuntime = "victoria-logs"
	// VLSingleResourceName is the name of the VLSingle resource.
	VLSingleResourceName = "victoria-logs"
)
