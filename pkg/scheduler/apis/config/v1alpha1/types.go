// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
)

const (
	// SameRegion Strategy determines a seed candidate for a shoot only if the cloud profile and region are identical
	SameRegion CandidateDeterminationStrategy = "SameRegion"
	// MinimalDistance Strategy determines a seed candidate for a shoot if the cloud profile are identical. Then chooses the seed with the minimal distance to the shoot.
	MinimalDistance CandidateDeterminationStrategy = "MinimalDistance"
	// Default Strategy is the default strategy to use when there is no configuration provided
	Default = SameRegion
	// SchedulerDefaultLockObjectNamespace is the default lock namespace for leader election.
	SchedulerDefaultLockObjectNamespace = "garden"
	// SchedulerDefaultLockObjectName is the default lock name for leader election.
	SchedulerDefaultLockObjectName = "gardener-scheduler-leader-election"
	// SchedulerDefaultConfigurationConfigMapNamespace is the namespace of the scheduler configuration config map
	SchedulerDefaultConfigurationConfigMapNamespace = "garden"
	// SchedulerDefaultConfigurationConfigMapName is the name of the scheduler configuration config map
	SchedulerDefaultConfigurationConfigMapName = "gardener-scheduler-configmap"
	// DefaultDiscoveryTTL is the default ttl for the cached discovery client.
	DefaultDiscoveryTTL = 10 * time.Second

	// LogLevelDebug is the debug log level, i.e. the most verbose.
	LogLevelDebug = "debug"
	// LogLevelInfo is the default log level.
	LogLevelInfo = "info"
	// LogLevelError is a log level where only errors are logged.
	LogLevelError = "error"

	// LogFormatJSON is the output type that produces a JSON object per log line.
	LogFormatJSON = "json"
	// LogFormatText outputs the log as human-readable text.
	LogFormatText = "text"
)

// Strategies defines all currently implemented SeedCandidateDeterminationStrategies
var Strategies = []CandidateDeterminationStrategy{SameRegion, MinimalDistance}

// CandidateDeterminationStrategy defines how seeds for shoots, that do not specify a seed explicitly, are being determined
type CandidateDeterminationStrategy string

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SchedulerConfiguration defines the configuration for the Gardener scheduler.
type SchedulerConfiguration struct {
	metav1.TypeMeta `json:",inline"`
	// ClientConnection specifies the kubeconfig file and client connection
	// settings for the proxy server to use when communicating with the apiserver.
	ClientConnection componentbaseconfigv1alpha1.ClientConnectionConfiguration `json:"clientConnection,omitempty"`
	// LeaderElection defines the configuration of leader election client.
	// +optional
	LeaderElection *componentbaseconfigv1alpha1.LeaderElectionConfiguration `json:"leaderElection,omitempty"`
	// LogLevel is the level/severity for the logs. Must be one of [info,debug,error].
	LogLevel string `json:"logLevel,omitempty"`
	// LogFormat is the output format for the logs. Must be one of [text,json].
	LogFormat string `json:"logFormat,omitempty"`
	// Server defines the configuration of the HTTP server. This is deprecated in favor of
	// HealthServer.
	Server ServerConfiguration `json:"server"`
	// Debugging holds configuration for Debugging related features.
	// +optional
	Debugging *componentbaseconfigv1alpha1.DebuggingConfiguration `json:"debugging,omitempty"`
	// Scheduler defines the configuration of the schedulers.
	Schedulers SchedulerControllerConfiguration `json:"schedulers"`
	// FeatureGates is a map of feature names to bools that enable or disable alpha/experimental
	// features. This field modifies piecemeal the built-in default values from
	// "github.com/gardener/gardener/pkg/scheduler/features/features.go".
	// Default: nil
	// +optional
	FeatureGates map[string]bool `json:"featureGates,omitempty"`
}

// SchedulerControllerConfiguration defines the configuration of the controllers.
type SchedulerControllerConfiguration struct {
	// BackupBucket defines the configuration of the BackupBucket controller.
	// +optional
	BackupBucket *BackupBucketSchedulerConfiguration `json:"backupBucket,omitempty"`
	// Shoot defines the configuration of the Shoot controller.
	// +optional
	Shoot *ShootSchedulerConfiguration `json:"shoot,omitempty"`
}

// BackupBucketSchedulerConfiguration defines the configuration of the BackupBucket to Seed
// scheduler.
type BackupBucketSchedulerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int `json:"concurrentSyncs"`
}

// ShootSchedulerConfiguration defines the configuration of the Shoot to Seed
// scheduler.
type ShootSchedulerConfiguration struct {
	// ConcurrentSyncs is the number of workers used for the controller to work on
	// events.
	ConcurrentSyncs int `json:"concurrentSyncs"`
	// Strategy defines how seeds for shoots, that do not specify a seed explicitly, are being determined
	Strategy CandidateDeterminationStrategy `json:"candidateDeterminationStrategy"`
}

// ServerConfiguration contains details for the HTTP(S) servers.
type ServerConfiguration struct {
	// HealthProbes is the configuration for serving the healthz and readyz endpoints.
	// +optional
	HealthProbes *Server `json:"healthProbes,omitempty"`
	// Metrics is the configuration for serving the metrics endpoint.
	// +optional
	Metrics *Server `json:"metrics,omitempty"`
}

// Server contains information for HTTP(S) server configuration.
type Server struct {
	// BindAddress is the IP address on which to listen for the specified port.
	BindAddress string `json:"bindAddress"`
	// Port is the port on which to serve unsecured, unauthenticated access.
	Port int `json:"port"`
}
