// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package constants

const (
	// VictoriaLogsPort is the port exposed by VictoriaLogs.
	VictoriaLogsPort = 9428
	// ServiceName is the name of the logging service.
	ServiceName = "logging"
	// PushEndpoint is the endpoint used by VictoriaLogs to receive logs.
	PushEndpoint = "/insert/opentelemetry/v1/logs"
	// ManagedResourceNameRuntime is the name of the managed resource which deploys VictoriaLogs.
	ManagedResourceNameRuntime = "victoria-logs"
	// VLSingleResourceName is the name of the VLSingle resource.
	VLSingleResourceName = "victoria-logs"
)
