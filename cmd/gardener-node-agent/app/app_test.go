// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"testing"

	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
)

func TestSanitizeRESTConfigError(t *testing.T) {
	tests := []struct {
		name     string
		input    error
		expected string
	}{
		{
			name:     "nil error",
			input:    nil,
			expected: "",
		},
		{
			name:     "dial tcp error",
			input:    fmt.Errorf("failed requesting kubeconfig: unable to create clientset: Get \"https://api.example.com\": dial tcp 10.0.0.1:443: i/o timeout"),
			expected: "network connection failed: dial tcp 10.0.0.1:443: i/o timeout",
		},
		{
			name:     "connection refused",
			input:    fmt.Errorf("failed to connect: connection refused"),
			expected: "API server connection refused - check if API server is accessible",
		},
		{
			name:     "i/o timeout",
			input:    fmt.Errorf("request failed: i/o timeout"),
			expected: "API server connection timeout - check network connectivity and firewall rules",
		},
		{
			name:     "no route to host",
			input:    fmt.Errorf("network error: no route to host"),
			expected: "no network route to API server - check routing configuration",
		},
		{
			name:     "certificate error",
			input:    fmt.Errorf("TLS handshake failed: x509: certificate signed by unknown authority"),
			expected: "TLS/certificate validation failed - check CA bundle configuration",
		},
		{
			name:     "tls error",
			input:    fmt.Errorf("tls: bad certificate"),
			expected: "TLS/certificate validation failed - check CA bundle configuration",
		},
		{
			name:     "401 unauthorized",
			input:    fmt.Errorf("API returned: 401 Unauthorized"),
			expected: "authentication failed - check bootstrap token validity",
		},
		{
			name:     "403 forbidden",
			input:    fmt.Errorf("API returned: 403 Forbidden"),
			expected: "authorization failed - check RBAC permissions for node-agent",
		},
		{
			name:     "generic error with token path",
			input:    fmt.Errorf("failed to read %s: file not found", nodeagentconfigv1alpha1.BootstrapTokenFilePath),
			expected: "unable to connect to Kubernetes API server: failed to read <bootstrap-token-path>: file not found",
		},
		{
			name:     "generic error with kubeconfig path",
			input:    fmt.Errorf("failed to parse %s: invalid format", nodeagentconfigv1alpha1.KubeconfigFilePath),
			expected: "unable to connect to Kubernetes API server: failed to parse <kubeconfig-path>: invalid format",
		},
		{
			name:     "generic unknown error",
			input:    fmt.Errorf("some unknown error"),
			expected: "unable to connect to Kubernetes API server: some unknown error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeRESTConfigError(tt.input)
			var resultStr string
			if result != nil {
				resultStr = result.Error()
			}

			if resultStr != tt.expected {
				t.Errorf("sanitizeRESTConfigError() = %q, want %q", resultStr, tt.expected)
			}
		})
	}
}
