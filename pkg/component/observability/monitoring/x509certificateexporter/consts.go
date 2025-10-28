package x509certificateexporter

import "time"
import monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

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
	// port on which the x509-certificate-exporter exposes metrics
	port = 9793
	// portName is the name of the port on which the x509-certificate-exporter exposes metrics and is scraped on
	portName = "metrics"
	// labelComponent is the component label value for the `role` label
	labelComponent = "x509-certificate-exporter"
	// defaultReplicas is the default number of replicas for the x509-certificate-exporter deployment
	defaultReplicas uint32 = 1
	// defaultCertCacheDuration is the default duration for which certificates are cached
	defaultCertCacheDuration = 24 * time.Hour
	// defaultKubeApiBurst is the default burst for the kube api client
	defaultKubeApiBurst uint32 = 30
	// defaultKubeApiRateLimit is the default rate limit for the kube api client
	defaultKubeApiRateLimit uint32 = 20
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
const (
	SuffixSeed    = "-seed"
	SuffixRuntime = "-runtime"
	SuffixShoot   = "-shoot"
)
