// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Important:
// This file is copied from https://github.com/open-telemetry/opentelemetry-operator/blob/v0.143.0/apis/v1beta1/networkpolicy.go.

package v1beta1

// NetworkPolicy defines the configuration for NetworkPolicy.
type NetworkPolicy struct {
	// Enable enables the NetworkPolicy.
	// The default value is taken from the operator feature-gate `--feature-gates=+operand.networkpolicy`.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Enable NetworkPolicy"
	Enabled *bool `json:"enabled,omitempty"`
}
