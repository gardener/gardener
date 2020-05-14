// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"

	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	// BackupBucketName is a constant for the name of bucket of object storage.
	BackupBucketName = "bucketName"

	// BackupSecretName defines the name of the secret containing the credentials which are required to
	// authenticate against the respective cloud provider (required to store the backups of Shoot clusters).
	BackupSecretName = "etcd-backup"

	// BasicAuthSecretName is the name of the secret containing basic authentication credentials for the kube-apiserver.
	BasicAuthSecretName = "kube-apiserver-basic-auth"

	// ChartPath is the path to the Helm charts.
	ChartPath = "charts"

	// CloudConfigPrefix is a constant for the prefix which is added to secret storing the original cloud config (which
	// is being downloaded from the cloud-config-downloader process)
	CloudConfigPrefix = "cloud-config"

	// CloudConfigFilePath is the path on the shoot worker nodes to which the operating system specific configuration
	// will be downloaded.
	CloudConfigFilePath = "/var/lib/cloud-config-downloader/downloads/cloud_config"

	// ConfirmationDeletion is an annotation on a Shoot and Project resources whose value must be set to "true" in order to
	// allow deleting the resource (if the annotation is not set any DELETE request will be denied).
	ConfirmationDeletion = "confirmation.gardener.cloud/deletion"

	// ConfirmationDeletionDeprecated is an annotation on a Shoot resource whose value must be set to "true" in order to
	// allow deleting the Shoot (if the annotation is not set any DELETE request will be denied).
	//
	// Deprecated: Use `ConfirmationDeletion` instead.
	ConfirmationDeletionDeprecated = "confirmation.garden.sapcloud.io/deletion"

	// ControllerManagerInternalConfigMapName is the name of the internal config map in which the Gardener controller
	// manager stores its configuration.
	ControllerManagerInternalConfigMapName = "gardener-controller-manager-internal-config"

	// DNSProviderDeprecated is the key for an annotation on a Kubernetes Secret object whose value must point to a valid
	// DNS provider.
	//
	// Deprecated: Use `DNSProvider` instead.
	DNSProviderDeprecated = "dns.garden.sapcloud.io/provider"

	// DNSDomainDeprecated is the key for an annotation on a Kubernetes Secret object whose value must point to a valid
	// domain name.
	//
	// Deprecated: Use `DNSDomain` instead.
	DNSDomainDeprecated = "dns.garden.sapcloud.io/domain"

	// DNSProvider is the key for an annotation on a Kubernetes Secret object whose value must point to a valid
	// DNS provider.
	DNSProvider = "dns.gardener.cloud/provider"

	// DNSDomain is the key for an annotation on a Kubernetes Secret object whose value must point to a valid
	// domain name.
	DNSDomain = "dns.gardener.cloud/domain"

	// DNSIncludeZones is the key for an annotation on a Kubernetes Secret object whose value must point to a list
	// of zones that shall be included.
	DNSIncludeZones = "dns.gardener.cloud/include-zones"

	// DNSExcludeZones is the key for an annotation on a Kubernetes Secret object whose value must point to a list
	// of zones that shall be excluded.
	DNSExcludeZones = "dns.gardener.cloud/exclude-zones"

	// EtcdRoleMain is the constant defining the role for main etcd storing data about objects in Shoot.
	EtcdRoleMain = "main"

	// EtcdRoleEvents is the constant defining the role for etcd storing events in Shoot.
	EtcdRoleEvents = "events"

	// EtcdEncryptionSecretName is the name of the shoot-specific secret which contains
	// that shoot's EncryptionConfiguration. The EncryptionConfiguration contains a key
	// which the shoot's apiserver uses for encrypting selected etcd content.
	// Should match charts/seed-controlplane/charts/kube-apiserver/templates/deployment.yaml
	EtcdEncryptionSecretName = "etcd-encryption-secret"

	// EtcdEncryptionSecretFileName is the name of the file within the EncryptionConfiguration
	// which is made available as volume mount to the shoot's apiserver.
	// Should match charts/seed-controlplane/charts/kube-apiserver/templates/deployment.yaml
	EtcdEncryptionSecretFileName = "encryption-configuration.yaml"

	// EtcdEncryptionChecksumAnnotationName is the name of the annotation with which to annotate
	// the EncryptionConfiguration secret to denote the checksum of the EncryptionConfiguration
	// that was used when last rewriting secrets.
	EtcdEncryptionChecksumAnnotationName = "shoot.gardener.cloud/etcd-encryption-configuration-checksum"

	// EtcdEncryptionChecksumLabelName is the name of the label which is added to the shoot
	// secrets after rewriting them to ensure that successfully rewritten secrets are not
	// (unnecessarily) rewritten during each reconciliation.
	EtcdEncryptionChecksumLabelName = "shoot.gardener.cloud/etcd-encryption-configuration-checksum"

	// EtcdEncryptionForcePlaintextAnnotationName is the name of the annotation with which to annotate
	// the EncryptionConfiguration secret to force the decryption of shoot secrets
	EtcdEncryptionForcePlaintextAnnotationName = "shoot.gardener.cloud/etcd-encryption-force-plaintext-secrets"

	// EtcdEncryptionEncryptedResourceSecrets is the name of the secret resource to be encrypted
	EtcdEncryptionEncryptedResourceSecrets = "secrets"

	// EtcdEncryptionKeyPrefix is the prefix for the key name of the EncryptionConfiguration's key
	EtcdEncryptionKeyPrefix = "key"

	// EtcdEncryptionKeySecretLen is the expected length in bytes of the EncryptionConfiguration's key
	EtcdEncryptionKeySecretLen = 32

	// GardenerDeletionProtected is a label on CustomResourceDefinitions indicating that the deletion is protected, i.e.
	// it must be confirmed with the `confirmation.gardener.cloud/deletion=true` annotation before a `DELETE` call
	// is accepted.
	GardenerDeletionProtected = "gardener.cloud/deletion-protected"

	// ETCDEncryptionConfigDataName is the name of ShootState data entry holding the current key and encryption state used to encrypt shoot resources
	ETCDEncryptionConfigDataName = "etcdEncryptionConfiguration"

	// GardenRoleDefaultDomain is the value of the GardenRole key indicating type 'default-domain'.
	GardenRoleDefaultDomain = "default-domain"

	// GardenRoleInternalDomain is the value of the GardenRole key indicating type 'internal-domain'.
	GardenRoleInternalDomain = "internal-domain"

	// GardenRoleAlertingSMTP is the value of the GardenRole key indicating type 'alerting-smtp'.
	GardenRoleAlertingSMTP = "alerting-smtp"

	// GardenRoleOpenVPNDiffieHellman is the value of the GardenRole key indicating type 'openvpn-diffie-hellman'.
	GardenRoleOpenVPNDiffieHellman = "openvpn-diffie-hellman"

	// GardenRoleGlobalMonitoring is the value of the GardenRole key indicating type 'global-monitoring'
	GardenRoleGlobalMonitoring = "global-monitoring"

	// GardenRoleAlerting is the value of GardenRole key indicating type 'alerting'.
	GardenRoleAlerting = "alerting"

	// GardenRoleHvpa is the value of GardenRole key indicating type 'hvpa'.
	GardenRoleHvpa = "hvpa"

	// GardenCreatedBy is the key for an annotation of a Shoot cluster whose value indicates contains the username
	// of the user that created the resource.
	GardenCreatedBy = "gardener.cloud/created-by"

	// GardenCreatedByDeprecated is the key for an annotation of a Shoot cluster whose value indicates contains the username
	// of the user that created the resource.
	//
	// Deprecated: Use `GardenCreatedBy` instead.
	GardenCreatedByDeprecated = "garden.sapcloud.io/createdBy"

	// GrafanaOperatorsPrefix is a constant for a prefix used for the operators Grafana instance.
	GrafanaOperatorsPrefix = "go"

	// GrafanaUsersPrefix is a constant for a prefix used for the users Grafana instance.
	GrafanaUsersPrefix = "gu"

	// GrafanaOperatorsRole is a constant for the operators role.
	GrafanaOperatorsRole = "operators"

	// GrafanaUsersRole is a constant for the users role.
	GrafanaUsersRole = "users"

	// PrometheusPrefix is a constant for a prefix used for the Prometheus instance.
	PrometheusPrefix = "p"

	// AlertManagerPrefix is a constant for a prefix used for the AlertManager instance.
	AlertManagerPrefix = "au"

	// KibanaPrefix is a constant for a prefix used for the Kibana instance.
	KibanaPrefix = "k"

	// IngressPrefix is the part of a FQDN which will be used to construct the domain name for an ingress controller of
	// a Shoot cluster. For example, when a Shoot specifies domain 'cluster.example.com', the ingress domain would be
	// '*.<IngressPrefix>.cluster.example.com'.
	IngressPrefix = "ingress"

	// APIServerPrefix is the part of a FQDN which will be used to construct the domain name for the kube-apiserver of
	// a Shoot cluster. For example, when a Shoot specifies domain 'cluster.example.com', the apiserver domain would be
	// 'api.cluster.example.com'.
	APIServerPrefix = "api"

	// InternalDomainKey is a key which must be present in an internal domain constructed for a Shoot cluster. If the
	// configured internal domain already contains it, it won't be added twice. If it does not contain it, it will be
	// appended.
	InternalDomainKey = "internal"

	// KubeControllerManagerServerName is the name of the kube-controller-manager server.
	KubeControllerManagerServerName = "kube-controller-manager-server"

	// KubeSchedulerServerName is the name of the kube-scheduler server.
	KubeSchedulerServerName = "kube-scheduler-server"

	// CoreDNSDeploymentName is the name of the coredns deployment.
	CoreDNSDeploymentName = "coredns"

	// VPNShootDeploymentName is the name of the vpn-shoot deployment.
	VPNShootDeploymentName = "vpn-shoot"

	// MetricsServerDeploymentName is the name of the metrics-server deployment.
	MetricsServerDeploymentName = "metrics-server"

	// KubeProxyDaemonSetName is the name of the kube-proxy daemon set.
	KubeProxyDaemonSetName = "kube-proxy"

	// NodeProblemDetectorDaemonSetName is the name of the node-problem-detector daemon set.
	NodeProblemDetectorDaemonSetName = "node-problem-detector"

	// BlackboxExporterDeploymentName is the name of the blackbox-exporter deployment.
	BlackboxExporterDeploymentName = "blackbox-exporter"

	// NodeExporterDaemonSetName is the name of the node-exporter daemon set.
	NodeExporterDaemonSetName = "node-exporter"

	// KibanaAdminIngressCredentialsSecretName is the name of the secret which holds admin credentials.
	KibanaAdminIngressCredentialsSecretName = "logging-ingress-credentials"

	// KubecfgUsername is the username for the token used for the kubeconfig the shoot.
	KubecfgUsername = "system:cluster-admin"

	// KubecfgSecretName is the name of the kubecfg secret.
	KubecfgSecretName = "kubecfg"

	// DependencyWatchdogExternalProbeSecretName is the name of the kubecfg secret with internal DNS for external access.
	DependencyWatchdogExternalProbeSecretName = "dependency-watchdog-external-probe"

	// DependencyWatchdogInternalProbeSecretName is the name of the kubecfg secret with cluster IP access.
	DependencyWatchdogInternalProbeSecretName = "dependency-watchdog-internal-probe"

	// DependencyWatchdogUserName is the user name of the dependency-watchdog.
	DependencyWatchdogUserName = "gardener.cloud:system:dependency-watchdog"

	// KubeAPIServerHealthCheck is a key for the kube-apiserver-health-check user.
	KubeAPIServerHealthCheck = "kube-apiserver-health-check"

	// StaticTokenSecretName is the name of the secret containing static tokens for the kube-apiserver.
	StaticTokenSecretName = "static-token"

	// FluentdEsStatefulSetName is the name of the fluentd-es stateful set.
	FluentdEsStatefulSetName = "fluentd-es"

	// ProjectPrefix is the prefix of namespaces representing projects.
	ProjectPrefix = "garden-"

	// ProjectName is they key of a label on namespaces whose value holds the project name.
	ProjectName = "project.gardener.cloud/name"

	// ProjectNameDeprecated is they key of a label on namespaces whose value holds the project name.
	//
	// Deprecated: Use `ProjectName` instead.
	ProjectNameDeprecated = "project.garden.sapcloud.io/name"

	// NamespaceProject is they key of an annotation on namespace whose value holds the project uid.
	NamespaceProject = "namespace.gardener.cloud/project"

	// NamespaceProjectDeprecated is they key of an annotation on namespace whose value holds the project uid.
	//
	// Deprecated: Use `NamespaceProject` instead.
	NamespaceProjectDeprecated = "namespace.garden.sapcloud.io/project"

	// ShootAlphaScalingAPIServerClass is a constant for an annotation on the shoot stating the initial API server class.
	// It influences the size of the initial resource requests/limits.
	// Possible values are [small, medium, large, xlarge, 2xlarge].
	// Note that this annotation is alpha and can be removed anytime without further notice. Only use it if you know
	// what you do.
	ShootAlphaScalingAPIServerClass = "alpha.kube-apiserver.scaling.shoot.gardener.cloud/class"

	// ShootExperimentalAddonKyma is a constant for an annotation on the shoot stating that Kyma shall be installed.
	// TODO: Just a temporary solution. Remove this in a future version once Kyma is moved out again.
	ShootExperimentalAddonKyma = "experimental.addons.shoot.gardener.cloud/kyma"

	// ShootExpirationTimestamp is an annotation on a Shoot resource whose value represents the time when the Shoot lifetime
	// is expired. The lifetime can be extended, but at most by the minimal value of the 'clusterLifetimeDays' property
	// of referenced quotas.
	ShootExpirationTimestamp = "shoot.gardener.cloud/expiration-timestamp"

	// ShootExpirationTimestampDeprecated is an annotation on a Shoot resource whose value represents the time when the Shoot lifetime
	// is expired. The lifetime can be extended, but at most by the minimal value of the 'clusterLifetimeDays' property
	// of referenced quotas.
	//
	// Deprecated: Use `ShootExpirationTimestamp` instead.
	ShootExpirationTimestampDeprecated = "shoot.garden.sapcloud.io/expirationTimestamp"

	// ShootNoCleanup is a constant for a label on a resource indicating the the Gardener cleaner should not delete this
	// resource when cleaning a shoot during the deletion flow.
	ShootNoCleanup = "shoot.gardener.cloud/no-cleanup"

	// ShootStatus is a constant for a label on a Shoot resource indicating that the Shoot's health.
	ShootStatus = "shoot.gardener.cloud/status"

	// ShootStatusDeprecated is a constant for a label on a Shoot resource indicating that the Shoot's health.
	//
	// Deprecated: Use `ShootStatus` instead.
	ShootStatusDeprecated = "shoot.garden.sapcloud.io/status"

	// ShootOperationDeprecated is a constant for an annotation on a Shoot in a failed state indicating that an operation shall be performed.
	//
	// Deprecated: Use `v1beta1constants.GardenerOperation` instead.
	ShootOperationDeprecated = "shoot.garden.sapcloud.io/operation"

	// ShootOperationMaintain is a constant for an annotation on a Shoot indicating that the Shoot maintenance shall be executed as soon as
	// possible.
	ShootOperationMaintain = "maintain"

	// FailedShootNeedsRetryOperation is a constant for an annotation on a Shoot in a failed state indicating that a retry operation should be triggered during the next maintenance time window.
	FailedShootNeedsRetryOperation = "maintenance.shoot.gardener.cloud/needs-retry-operation"

	// ShootOperationRotateKubeconfigCredentials is a constant for an annotation on a Shoot indicating that the credentials contained in the
	// kubeconfig that is handed out to the user shall be rotated.
	ShootOperationRotateKubeconfigCredentials = "rotate-kubeconfig-credentials"

	// ShootTasks is a constant for an annotation on a Shoot which states that certain tasks should be done.
	ShootTasks = "shoot.gardener.cloud/tasks"

	// ShootTasksDeprecated is a constant for an annotation on a Shoot which states that certain tasks should be done.
	//
	// Deprecated: Use `ShootTasks` instead.
	ShootTasksDeprecated = "shoot.garden.sapcloud.io/tasks"

	// ShootTaskDeployInfrastructure is a name for a Shoot's infrastructure deployment task.
	ShootTaskDeployInfrastructure = "deployInfrastructure"

	// ShootTaskRestartControlPlanePods is a name for a Shoot task which is dedicated to restart related control plane pods.
	ShootTaskRestartControlPlanePods = "restartControlPlanePods"

	// ShootOperationRetry is a constant for an annotation on a Shoot indicating that a failed Shoot reconciliation shall be retried.
	ShootOperationRetry = "retry"

	// ShootOperationReconcile is a constant for an annotation on a Shoot indicating that a Shoot reconciliation shall be triggered.
	ShootOperationReconcile = "reconcile"

	// ShootSyncPeriod is a constant for an annotation on a Shoot which may be used to overwrite the global Shoot controller sync period.
	// The value must be a duration. It can also be used to disable the reconciliation at all by setting it to 0m. Disabling the reconciliation
	// does only mean that the period reconciliation is disabled. However, when the Gardener is restarted/redeployed or the specification is
	// changed then the reconciliation flow will be executed.
	ShootSyncPeriod = "shoot.gardener.cloud/sync-period"

	// ShootSyncPeriodDeprecated is a constant for an annotation on a Shoot which may be used to overwrite the global Shoot controller sync period.
	// The value must be a duration. It can also be used to disable the reconciliation at all by setting it to 0m. Disabling the reconciliation
	// does only mean that the period reconciliation is disabled. However, when the Gardener is restarted/redeployed or the specification is
	// changed then the reconciliation flow will be executed.
	//
	// Deprecated: Use `ShootSyncPeriod` instead.
	ShootSyncPeriodDeprecated = "shoot.garden.sapcloud.io/sync-period"

	// ShootIgnore is a constant for an annotation on a Shoot which may be used to tell the Gardener that the Shoot with this name should be
	// ignored completely. That means that the Shoot will never reach the reconciliation flow (independent of the operation (create/update/
	// delete)).
	ShootIgnore = "shoot.gardener.cloud/ignore"

	// ShootIgnoreDeprecated is a constant for an annotation on a Shoot which may be used to tell the Gardener that the Shoot with this name should be
	// ignored completely. That means that the Shoot will never reach the reconciliation flow (independent of the operation (create/update/
	// delete)).
	//
	// Deprecated: Use `ShootIgnore` instead.
	ShootIgnoreDeprecated = "shoot.garden.sapcloud.io/ignore"

	// GardenerResourceManagerImageName is the name of the GardenerResourceManager image.
	GardenerResourceManagerImageName = "gardener-resource-manager"

	// GardenerSeedAdmissionControllerImageName is the name of the GardenerSeedAdmissionController image.
	GardenerSeedAdmissionControllerImageName = "gardener-seed-admission-controller"

	// CoreDNSImageName is the name of the CoreDNS image.
	CoreDNSImageName = "coredns"

	// NodeProblemDetectorImageName is the name of the node-problem-detector image.
	NodeProblemDetectorImageName = "node-problem-detector"

	// KubeAPIServerImageName is the name of the kube-apiserver image.
	KubeAPIServerImageName = "kube-apiserver"

	// KubeControllerManagerImageName is the name of the kube-controller-manager image.
	KubeControllerManagerImageName = "kube-controller-manager"

	// KubeSchedulerImageName is the name of the kube-scheduler image.
	KubeSchedulerImageName = "kube-scheduler"

	// KubeProxyImageName is the name of the kube-proxy image.
	KubeProxyImageName = "kube-proxy"

	// HyperkubeImageName is the name of the hyperkube image (used for kubectl + kubelet on the worker nodes).
	HyperkubeImageName = "hyperkube"

	// MetricsServerImageName is the name of the MetricsServer image.
	MetricsServerImageName = "metrics-server"

	// VPNShootImageName is the name of the VPNShoot image.
	VPNShootImageName = "vpn-shoot"

	// VPNSeedImageName is the name of the VPNSeed image.
	VPNSeedImageName = "vpn-seed"

	// NodeExporterImageName is the name of the NodeExporter image.
	NodeExporterImageName = "node-exporter"

	// KubernetesDashboardImageName is the name of the kubernetes-dashboard image.
	KubernetesDashboardImageName = "kubernetes-dashboard"

	// KubernetesDashboardMetricsScraperImageName is the name of the kubernetes-dashboard-metrics-scraper image.
	KubernetesDashboardMetricsScraperImageName = "kubernetes-dashboard-metrics-scraper"

	// BusyboxImageName is the name of the Busybox image.
	BusyboxImageName = "busybox"

	// NginxIngressControllerImageName is the name of the NginxIngressController image.
	NginxIngressControllerImageName = "nginx-ingress-controller"

	// IngressDefaultBackendImageName is the name of the IngressDefaultBackend image.
	IngressDefaultBackendImageName = "ingress-default-backend"

	// ClusterAutoscalerImageName is the name of the ClusterAutoscaler image.
	ClusterAutoscalerImageName = "cluster-autoscaler"

	// AlertManagerImageName is the name of the AlertManager image.
	AlertManagerImageName = "alertmanager"

	// ConfigMapReloaderImageName is the name of the ConfigMapReloader image.
	ConfigMapReloaderImageName = "configmap-reloader"

	// GrafanaImageName is the name of the Grafana image.
	GrafanaImageName = "grafana"

	// PrometheusImageName is the name of the Prometheus image.
	PrometheusImageName = "prometheus"

	// BlackboxExporterImageName is the name of the BlackboxExporter image.
	BlackboxExporterImageName = "blackbox-exporter"

	// KubeStateMetricsImageName is the name of the KubeStateMetrics image.
	KubeStateMetricsImageName = "kube-state-metrics"

	// EtcdDruidImageName is the name of Etcd Druid image
	EtcdDruidImageName = "etcd-druid"

	// PauseContainerImageName is the name of the PauseContainer image.
	PauseContainerImageName = "pause-container"

	// ElasticsearchImageName is the name of the Elastic-Search image used for logging
	ElasticsearchImageName = "elasticsearch-oss"

	// ElasticsearchMetricsExporterImageName is the name of the metrics exporter image used to fetch elasticsearch metrics.
	ElasticsearchMetricsExporterImageName = "elasticsearch-metrics-exporter"

	// ElasticsearchSearchguardImageName is the name of the Elastic-Search image with installed searchguard plugin used for logging
	ElasticsearchSearchguardImageName = "elasticsearch-searchguard-oss"

	// CuratorImageName is the name of the curator image used to alter the Elastic-search logs
	CuratorImageName = "curator-es"

	// KibanaImageName is the name of the Kibana image used for logging  UI
	KibanaImageName = "kibana-oss"

	// SearchguardImageName is the name of the Searchguard image used for updating the users and roles
	SearchguardImageName = "sg-sgadmin"

	// FluentdEsImageName is the image of the Fluentd image used for logging
	FluentdEsImageName = "fluentd-es"

	// FluentBitImageName is the image of Fluent-bit image
	FluentBitImageName = "fluent-bit"

	// AlpineImageName is the name of alpine image
	AlpineImageName = "alpine"

	// AlpineIptablesImageName is the name of the alpine image with pre-installed iptable rules
	AlpineIptablesImageName = "alpine-iptables"

	// SeedSpecHash is a constant for a label on `ControllerInstallation`s (similar to `pod-template-hash` on `Pod`s).
	SeedSpecHash = "seed-spec-hash"

	// RegistrationSpecHash is a constant for a label on `ControllerInstallation`s (similar to `pod-template-hash` on `Pod`s).
	RegistrationSpecHash = "registration-spec-hash"

	// VpaAdmissionControllerImageName is the name of the vpa-admission-controller image
	VpaAdmissionControllerImageName = "vpa-admission-controller"

	// VpaRecommenderImageName is the name of the vpa-recommender image
	VpaRecommenderImageName = "vpa-recommender"

	// VpaUpdaterImageName is the name of the vpa-updater image
	VpaUpdaterImageName = "vpa-updater"

	// VpaExporterImageName is the name of the vpa-exporter image
	VpaExporterImageName = "vpa-exporter"

	// HvpaControllerImageName is the name of the hvpa-controller image
	HvpaControllerImageName = "hvpa-controller"

	// DependencyWatchdogImageName is the name of the dependency-watchdog image
	DependencyWatchdogImageName = "dependency-watchdog"

	// ServiceAccountSigningKeySecretDataKey is the data key of a signing key Kubernetes secret.
	ServiceAccountSigningKeySecretDataKey = "signing-key"

	// ControlPlaneWildcardCert is the value of the GardenRole key indicating type 'controlplane-cert'.
	// It refers to a wildcard tls certificate which can be used for services exposed under the corresponding domain.
	ControlPlaneWildcardCert = "controlplane-cert"

	// AlertManagerTLS is the name of the secret resource which holds the TLS certificate for Alert Manager.
	AlertManagerTLS = "alertmanager-tls"
	// GrafanaTLS is the name of the secret resource which holds the TLS certificate for Grafana.
	GrafanaTLS = "grafana-tls"
	// PrometheusTLS is the name of the secret resource which holds the TLS certificate for Prometheus.
	PrometheusTLS = "prometheus-tls"
	// KibanaTLS is the name of the secret resource which holds the TLS certificate for Kibana.
	KibanaTLS = "kibana-tls"
	// EtcdServerTLS is the name of the secret resource which holds TLS server certificate of Etcd
	EtcdServerTLS = "etcd-server-cert"
	// EtcdClientTLS is the name of the secret resource which holds TLS client certificate of Etcd
	EtcdClientTLS = "etcd-client-tls"

	// EndUserCrtValidity is the time period a user facing certificate is valid.
	EndUserCrtValidity = 730 * 24 * time.Hour // ~2 years, see https://support.apple.com/en-us/HT210176
)

var (
	// RequiredControlPlaneDeployments is a set of the required shoot control plane deployments
	// running in the seed.
	RequiredControlPlaneDeployments = sets.NewString(
		v1beta1constants.DeploymentNameGardenerResourceManager,
		v1beta1constants.DeploymentNameKubeAPIServer,
		v1beta1constants.DeploymentNameKubeControllerManager,
		v1beta1constants.DeploymentNameKubeScheduler,
	)

	// RequiredControlPlaneEtcds is a set of the required shoot control plane etcds
	// running in the seed.
	RequiredControlPlaneEtcds = sets.NewString(
		v1beta1constants.ETCDMain,
		v1beta1constants.ETCDEvents,
	)

	// RequiredSystemComponentDeployments is a set of the required system components.
	RequiredSystemComponentDeployments = sets.NewString(
		CoreDNSDeploymentName,
		VPNShootDeploymentName,
		MetricsServerDeploymentName,
	)

	// RequiredSystemComponentDaemonSets is a set of the required shoot control plane daemon sets.
	RequiredSystemComponentDaemonSets = sets.NewString(
		KubeProxyDaemonSetName,
		NodeProblemDetectorDaemonSetName,
	)

	// RequiredMonitoringSeedDeployments is a set of the required seed monitoring deployments.
	RequiredMonitoringSeedDeployments = sets.NewString(
		v1beta1constants.DeploymentNameGrafanaOperators,
		v1beta1constants.DeploymentNameGrafanaUsers,
		v1beta1constants.DeploymentNameKubeStateMetricsSeed,
		v1beta1constants.DeploymentNameKubeStateMetricsShoot,
	)

	// RequiredMonitoringShootDeployments is a set of the required shoot monitoring deployments.
	RequiredMonitoringShootDeployments = sets.NewString(
		BlackboxExporterDeploymentName,
	)

	// RequiredMonitoringShootDaemonSets is a set of the required shoot monitoring daemon sets.
	RequiredMonitoringShootDaemonSets = sets.NewString(
		NodeExporterDaemonSetName,
	)

	// RequiredLoggingStatefulSets is a set of the required logging stateful sets.
	RequiredLoggingStatefulSets = sets.NewString(
		v1beta1constants.StatefulSetNameElasticSearch,
	)

	// RequiredLoggingDeployments is a set of the required logging deployments.
	RequiredLoggingDeployments = sets.NewString(
		v1beta1constants.DeploymentNameKibana,
	)
)
