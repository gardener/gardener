// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	corev1 "k8s.io/api/core/v1"
)

// Config is the dashboard config structure.
type Config struct {
	Port               int32   `yaml:"port"`
	LogFormat          string  `yaml:"logFormat"`
	LogLevel           string  `yaml:"logLevel"`
	APIServerURL       string  `yaml:"apiServerUrl"`
	APIServerCAData    *string `yaml:"apiServerCaData,omitempty"`
	MaxRequestBodySize string  `yaml:"maxRequestBodySize"`

	ReadinessProbe        ReadinessProbe         `yaml:"readinessProbe"`
	UnreachableSeeds      UnreachableSeeds       `yaml:"unreachableSeeds"`
	ContentSecurityPolicy *ContentSecurityPolicy `yaml:"contentSecurityPolicy,omitempty"`
	Terminal              *Terminal              `yaml:"terminal,omitempty"`
	OIDC                  *OIDC                  `yaml:"oidc,omitempty"`
	GitHub                *GitHub                `yaml:"gitHub,omitempty"`
	Frontend              map[string]any         `yaml:"frontend,omitempty"`
}

// ReadinessProbe is the readiness probe configuration.
type ReadinessProbe struct {
	PeriodSeconds int `yaml:"periodSeconds"`
}

// UnreachableSeeds is the configuration for unreachable seeds.
type UnreachableSeeds struct {
	MatchLabels map[string]string `yaml:"matchLabels"`
}

// ContentSecurityPolicy is the configuration for the content security policy.
type ContentSecurityPolicy struct {
	ConnectSources []string `yaml:"connectSrc"`
}

// Terminal is the configuration for the terminals.
type Terminal struct {
	Container                  TerminalContainer                   `yaml:"container"`
	ContainerImageDescriptions []TerminalContainerImageDescription `yaml:"containerImageDescriptions"`
	GardenTerminalHost         TerminalGardenHost                  `yaml:"gardenTerminalHost"`
	Garden                     TerminalGarden                      `yaml:"garden"`
}

// TerminalContainer is the configuration for a terminal container.
type TerminalContainer struct {
	Image string `yaml:"image"`
}

// TerminalContainerImageDescription is the configuration for terminal image descriptions.
type TerminalContainerImageDescription struct {
	Image       string `yaml:"image"`
	Description string `yaml:"description"`
}

// TerminalGardenHost is the configuration for the garden terminal host.
type TerminalGardenHost struct {
	SeedRef string `yaml:"seedRef"`
}

// TerminalGarden is the configuration for the garden terminals.
type TerminalGarden struct {
	OperatorCredentials TerminalOperatorCredentials `yaml:"operatorCredentials"`
}

// TerminalOperatorCredentials is the configuration for the operator credentials for terminals.
type TerminalOperatorCredentials struct {
	ServiceAccountRef corev1.SecretReference `yaml:"serviceAccountRef"`
}

// OIDC is the OIDC configuration.
type OIDC struct {
	Issuer             string     `yaml:"issuer"`
	SessionLifetime    int64      `yaml:"sessionLifetime"`
	RedirectURIs       []string   `yaml:"redirect_uris"`
	Scope              string     `yaml:"scope"`
	RejectUnauthorized bool       `yaml:"rejectUnauthorized"`
	Public             OIDCPublic `yaml:"public"`
	CA                 string     `yaml:"ca,omitempty"`
}

// OIDCPublic is the public OIDC configuration.
type OIDCPublic struct {
	ClientID string `yaml:"clientId"`
	UsePKCE  bool   `yaml:"usePKCE"`
}

// GitHub is the GitHub configuration.
type GitHub struct {
	APIURL              string `yaml:"apiUrl"`
	Org                 string `yaml:"org"`
	Repository          string `yaml:"repository"`
	PollIntervalSeconds *int64 `yaml:"pollIntervalSeconds,omitempty"`
	SyncThrottleSeconds int    `yaml:"syncThrottleSeconds"`
	SyncConcurrency     int    `yaml:"syncConcurrency"`
}
