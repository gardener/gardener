// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package x509certificateexporter

import (
	"time"

	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type podTypeLabelValues string

const (
	// inClusterCertificateLabelValue specifies pod is part of the k8s api monitoring deployment
	inClusterCertificateLabelValue podTypeLabelValues = "api"
	// nodeCertificateLabelValue specifies pod is part of the worker node monitoring daemonset
	nodeCertificateLabelValue podTypeLabelValues = "node"
	// Label name to help determine which part of the monitoring component is specified
	certificateSourceLabelName string = "certificate-source"
)

// commonExporterConfigs holds common configuration options for both in-cluster and node exporters
type commonExporterConfigs struct {
	// TrimComponents specifies how much of the len should be removed from the metrics
	TrimComponents *uint32 `yaml:"trimComponents,omitempty"`
	// ExposeRelativeMetrics flag for the exporter
	ExposeRelativeMetrics bool `yaml:"exposeRelativeMetrics,omitempty"`
	// ExposeExpiryMetrics flag for the exporter
	ExposePerCertErrorMetrics bool `yaml:"exposePerCertErrorMetrics,omitempty"`
	// ExposeExpiryMetrics flag for the exporter
	ExposeLabelsMetrics bool `yaml:"exposeLabelsMetrics,omitempty"`
}

// inClusterConfig holds configuration options for in-cluster x509 certificate monitoring
type inClusterConfig struct {
	commonExporterConfigs
	// Enabled specifies if the component is enabled
	Enabled bool `yaml:"enabled,omitempty"`
	// SecretTypes specifies the secret types to monitor
	SecretTypes []string `yaml:"secretTypes,omitempty"`
	// ConfigMapKeys specifies the config map keys to monitor
	ConfigMapKeys []string `yaml:"configMapKeys,omitempty"`
	// IncludeLabels includes labels, similar to the namespaces vars.
	IncludeLabels map[string]string `yaml:"includeLabels,omitempty"`
	// ExcludeLabels excludes labels, similar to the namespaces vars.
	ExcludeLabels map[string]string `yaml:"excludeLabels,omitempty"`
	// IncludeNamespaces are namespaces from which secrets are monitored.
	IncludeNamespaces []string `yaml:"includeNamespaces,omitempty"`
	// ExcludeNamespaces namespaces from which secrets are not monitored.
	ExcludeNamespaces []string `yaml:"excludeNamespaces,omitempty"`
	// Replicas is the number of replicas for the deployment of the incluster monitoring service
	Replicas *uint32 `yaml:"replicas,omitempty"`
	// MaxCacheDuration is the maximum duration to cache certificate data
	MaxCacheDuration time.Duration `yaml:"maxCacheDuration,omitempty"`
	// KubeAPIRateLimit is the rate limit for the kubernetes api calls
	KubeAPIRateLimit *uint32 `yaml:"kubeApiRateLimit,omitempty"`
	// KubeAPIBurst is the burst for the kubernetes api calls
	KubeAPIBurst *uint32 `yaml:"kubeApiBurst,omitempty"`
}

type monitorableMount struct {
	// Path is the mount path within the pod
	Path string `yaml:"path"`
	// WatchKubeconfigs is a list of kubeconfigs passed to the exporter
	WatchKubeconfigs []string `yaml:"watchKubeconfigs,omitempty"`
	// WatchCertificates is a list of certificate paths passed to the exporter
	WatchCertificates []string `yaml:"watchCertificates,omitempty"`
	// WatchDirs is a list of directories to watch for certificates
	WatchDirs []string `yaml:"watchDirs,omitempty"`
}

// workerGroup holds configuration options for a single worker group x509 certificate monitoring
type workerGroup struct {
	commonExporterConfigs
	// NameSuffix is attached to the daemonset name and related resources
	NameSuffix string `yaml:"nameSuffix,omitempty"`
	// Selector is the label selector to identify the worker nodes
	Selector *metav1.LabelSelector `yaml:"selectoroomitempty"`
	// Mounts is a map of mounts and the monitored resources within
	Mounts map[string]monitorableMount `yaml:"mounts"`
}

// workerGroupsConfig is a list of worker group configurations
// which brings the configurations for different node pools
type workerGroupsConfig []workerGroup

type prometheusRuleSeverity string

const (
	infoSeverity     prometheusRuleSeverity = "info"
	warningSeverity  prometheusRuleSeverity = "warning"
	criticalSeverity prometheusRuleSeverity = "critical"
	blockerSeverity  prometheusRuleSeverity = "blocker"
)

type alertingConfig struct {
	// CertificateRenewalDays specifies days before certificate expires that we will get an alert
	// specifying we need to renew
	CertificateRenewalDays uint `yaml:"certificateRenewalDays,omitempty"`
	// CertificateExpirationDays specifies days before certificate expires that we will get an alert
	CertificateExpirationDays uint `yaml:"certificateExpirationDays,omitempty"`
	// ReadErrorsSeverity is the severity level for read errors alerts
	ReadErrorsSeverity prometheusRuleSeverity `yaml:"readErrorsSeverity,omitempty"`
	// CertificateErrorsSeverity is the severity level for certificate errors alerts
	CertificateErrorsSeverity prometheusRuleSeverity `yaml:"certificateErrorsSeverity,omitempty"`
	// RenewalSeverity is the severity level for certificate renewal alerts
	RenewalSeverity prometheusRuleSeverity `yaml:"renewalSeverity,omitempty"`
	// ExpirationSeverity is the severity level for certificate expiration alerts
	ExpirationSeverity prometheusRuleSeverity `yaml:"expirationSeverity,omitempty"`
	// ExpiresTodaySeverity is the severity level for certificate expires today alerts
	ExpiresTodaySeverity prometheusRuleSeverity `yaml:"expiresTodaySeverity,omitempty"`
	// DurationForAlertEvaluation is the duration over which the alert is evaluated
	DurationForAlertEvaluation monitoringv1.Duration `yaml:"durationForAlertEvaluation,omitempty"`
	// PrometheusRuleName is the name of the PrometheusRule resource
	PrometheusRuleName string `yaml:"prometheusRuleName,omitempty"`
}

type x509certificateExporterConfig struct {
	inCluster    inClusterConfig    `yaml:"inCluster,omitempty"`
	workerGroups workerGroupsConfig `yaml:"workerGroups,omitempty"`
	alerting     alertingConfig     `yaml:"alertingConfig,omitempty"`
}

// Configurations for the x509 certificate exporter deploys
type Values struct {
	// Image sets container image.
	Image string
	// PriorityClassName is the name of the priority class.
	PriorityClassName string
	// NameSuffix is attached to the deployment name and related resources.
	NameSuffix string
	// PrometheusInstance is the label for the prometheus instance that will pull metrics from the exporter
	PrometheusInstance string
	// ConfigData is the configuration data for the x509-certificate-exporter
	ConfigData []byte
}

type x509CertificateExporter struct {
	client         client.Client
	secretsManager secretsmanager.Interface
	namespace      string
	values         Values
	conf           *x509certificateExporterConfig
}
