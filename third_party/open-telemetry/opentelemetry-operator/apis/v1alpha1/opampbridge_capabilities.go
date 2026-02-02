// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Important:
// This file is copied from https://github.com/open-telemetry/opentelemetry-operator/blob/v0.143.0/apis/v1alpha1/opembridge_capabilities.go.

package v1alpha1

type (
	// OpAMPBridgeCapability represents capability supported by OpAMP Bridge.
	// +kubebuilder:validation:Enum=AcceptsRemoteConfig;ReportsEffectiveConfig;ReportsOwnTraces;ReportsOwnMetrics;ReportsOwnLogs;AcceptsOpAMPConnectionSettings;AcceptsOtherConnectionSettings;AcceptsRestartCommand;ReportsHealth;ReportsRemoteConfig
	OpAMPBridgeCapability string
)

const (
	OpAMPBridgeCapabilityReportsStatus                  OpAMPBridgeCapability = "ReportsStatus"
	OpAMPBridgeCapabilityAcceptsRemoteConfig            OpAMPBridgeCapability = "AcceptsRemoteConfig"
	OpAMPBridgeCapabilityReportsEffectiveConfig         OpAMPBridgeCapability = "ReportsEffectiveConfig"
	OpAMPBridgeCapabilityReportsOwnTraces               OpAMPBridgeCapability = "ReportsOwnTraces"
	OpAMPBridgeCapabilityReportsOwnMetrics              OpAMPBridgeCapability = "ReportsOwnMetrics"
	OpAMPBridgeCapabilityReportsOwnLogs                 OpAMPBridgeCapability = "ReportsOwnLogs"
	OpAMPBridgeCapabilityAcceptsOpAMPConnectionSettings OpAMPBridgeCapability = "AcceptsOpAMPConnectionSettings"
	OpAMPBridgeCapabilityAcceptsOtherConnectionSettings OpAMPBridgeCapability = "AcceptsOtherConnectionSettings"
	OpAMPBridgeCapabilityAcceptsRestartCommand          OpAMPBridgeCapability = "AcceptsRestartCommand"
	OpAMPBridgeCapabilityReportsHealth                  OpAMPBridgeCapability = "ReportsHealth"
	OpAMPBridgeCapabilityReportsRemoteConfig            OpAMPBridgeCapability = "ReportsRemoteConfig"
)
