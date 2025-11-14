package x509certificateexporter

import (
	"time"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
)

// Private consts
// General
const (
	containerName       = "x509-certificate-exporter"
	managedResourceName = "x509-certificate-exporter"
	// promRuleName is the suffix for the PrometheusRule resource name, prefix is target
	promRuleName                 = "-x509-certificate-exporter"
	inClusterManagedResourceName = "x509-certificate-exporter"
	nodeManagedResourceName      = "x509-certificate-exporter-node"
	clusterRoleName              = "gardener-cloud:x509-certificate-exporter"
	clusterRoleBindingName       = "gardener-cloud:x509-certificate-exporter"
	// portName is the name of the port on which the x509-certificate-exporter exposes metrics and is scraped on
	portName = "metrics"
	// labelComponent is the component label value for the `role` label
	labelComponent = "x509-certificate-exporter"
	// defaultReplicas is the default number of replicas for the x509-certificate-exporter deployment
	defaultReplicas uint32 = 1
	// defaultCertCacheDuration is the default duration for which certificates are cached
	defaultCertCacheDuration = 24 * time.Hour
	// defaultKubeAPIBurst is the default burst for the kube api client
	defaultKubeAPIBurst uint32 = 30
	// defaultKubeAPIRateLimit is the default rate limit for the kube api client
	defaultKubeAPIRateLimit uint32 = 20
)

// Alerting consts
const (
	// defaultCertificateRenewalDays is the default number of days before expiration that will trigger a warning alert
	defaultCertificateRenewalDays = 14
	// defaultCertificateExpirationDays is the default number of days before expiration that will trigger a critical alert
	defaultCertificateExpirationDays = 7

	defaultReadErrorsSeverity         = infoSeverity
	defaultCertificateErrorsSeverity  = infoSeverity
	defaultRenewalSeverity            = warningSeverity
	defaultExpirationSeverity         = criticalSeverity
	defaultExpiresTodaySeverity       = blockerSeverity
	defaultSeverityKey                = "severity"
	defaultDurationForAlertEvaluation = monitoringv1.Duration("15m")
	defaultPrometheusRuleName         = "x509-certificate-exporter.rules"
)

// Public const
// Generall
const (
	SuffixSeed    = "-seed"
	SuffixRuntime = "-runtime"
	SuffixShoot   = "-shoot"
	// Port on which the x509-certificate-exporter exposes metrics
	Port = 9793
)

// Errors
const (
	// ErrUnsuportedClusterType is returned when new config is called for seed/shoot cluster
	ErrUnsuportedClusterType Error = "x509CertificateExporter is currently supported only on the runtime cluster"
	// ErrInvalidExporterConfigFormat is returned when the config format is innapropritate
	ErrInvalidExporterConfigFormat Error = "failed to unmarshal x509certificateexporter config"
	// ErrConfigValidationFailed is returned when config validation fails
	ErrConfigValidationFailed Error = "x509certificateexporter config validation failed"
	// ErrEmptyExporterConfig is returned when the config is empty
	ErrEmptyExporterConfig Error = "at least one of inCluster or workerGroups must be enabled"
	// ErrInClusterConfig is returned when in-cluster config is invalid
	ErrInClusterConfig Error = "in-cluster x509certificateexporter config is invalid"
	// ErrAlertingConfig is returned when alerting config is invalid
	ErrAlertingConfig Error = "alerting x509certificateexporter config is invalid"

	// ErrInvalidSeverity is returned when an invalid severity level is provided
	ErrInvalidSeverity Error = "invalid severity level provided"
	// ErrInvalidExpirationRenewalConf combination of expiration alert date and renewal alert date is invalid
	ErrInvalidExpirationRenewalConf Error = "certificateRenewalDays must be greater than or equal to certificateExpirationDays"

	// ErrWorkerGroupsConfig is returned when workergroups config is invalid
	ErrWorkerGroupsConfig Error = "Workergroups x509certificateexporter config is invalid"
	// ErrNoConfigMapKeyOrSecretTypes is returned when the incluster config does not have both configmap keys and secret types specified
	ErrNoConfigMapKeyOrSecretTypes Error = "at least one of secretTypes or configMapKeys must be specified when inCluster monitoring is enabled"
	// ErrWorkerGroupInvalid is returned when a workergroup config is invalid
	ErrWorkerGroupInvalid Error = "workerGroups validation errors"
	// ErrWorkerGroupMissingMount is returned when a workergroup is missing mount definitions
	ErrWorkerGroupMissingMount Error = "worker group must have at least one mount defined"
	// ErrMultipleGroupsNoSelectorOrSuffix is returned when multiple workergroups are defined but some of them miss name suffix or selector
	ErrMultipleGroupsNoSelectorOrSuffix Error = "multiple worker groups defined, but at least one is missing a node selector"
	// ErrHostPathNotAbsolute is returned when a relative path is passed as mount path
	ErrHostPathNotAbsolute Error = "host path is not an absolute path"
	// ErrMountPathEmpty is returned when mount path is empty
	ErrMountPathEmpty Error = "mount path is empty"
	// ErrMountPathNotAbsolute is returned when a relative path is passed as mount path
	ErrMountPathNotAbsolute Error = "mount path is not an absolute path"
	// ErrMountValidation is returned when a mount definition is invalid
	ErrMountValidation Error = "mount validation error"
	// ErrNoMonitorableFiles is returned when there are no files specified to monitor
	ErrNoMonitorableFiles Error = "at least one of watchKubeconfigs, watchCertificates, or watchDirs must be specified"
	// ErrWatchedFileNotAbsolutePath is returned when the monitored path is not an absolute path
	ErrWatchedFileNotAbsolutePath Error = "filepath is not an absolute path"
	// ErrMountValidationErrors is returned when there are multiple mount validation errors
	ErrMountValidationErrors Error = "mount validation errors"

	// ErrEmptyConfigMapKey is returned when the configmap key configured is empty
	ErrEmptyConfigMapKey Error = "config map key cannot be empty"
	// ErrKeyIsIllegal is returned when an illegal key is configured as a key in the configmap
	ErrKeyIsIllegal Error = "invalid configmap key"
	// ErrConfigMapMaxKeyLenght is returned when the configmap key is too long
	ErrConfigMapMaxKeyLenght Error = "config map key exceeds maximum length of 253 characrets"
	// ErrIncludeLabelsInvalid is returned when the labels set is invalid
	ErrIncludeLabelsInvalid Error = "includeLabels has invalid key or value"
	// ErrInvalidNamespace is returned when the NS argiment to the include/exclude namespaces is invalid
	ErrInvalidNamespace Error = "namespace is invalid"
	// ErrInvalidSecretType is returned when a secret type is invalid
	ErrInvalidSecretType Error = "invalid secret type for x509certificateexporter"
)
