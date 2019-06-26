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
	"fmt"
	"path/filepath"

	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	// AlertManagerStatefulSetName is the name of the alertmanager stateful set.
	AlertManagerStatefulSetName = "alertmanager"

	// BackupBucketName is a constant for the name of bucket of object storage.
	BackupBucketName = "bucketName"

	// BackupSecretName defines the name of the secret containing the credentials which are required to
	// authenticate against the respective cloud provider (required to store the backups of Shoot clusters).
	BackupSecretName = "etcd-backup"

	// BackupInfrastructureForceDeletion is a constant for an annotation on a Backupinfrastructure indicating that it should be force deleted.
	BackupInfrastructureForceDeletion = "backupinfrastructure.garden.sapcloud.io/force-deletion"

	// BackupInfrastructureOperation is a constant for an annotation on a Backupinfrastructure indicating that an operation shall be performed.
	BackupInfrastructureOperation = "backupinfrastructure.garden.sapcloud.io/operation"

	// BackupInfrastructureReconcile is a constant for an annotation on a Backupinfrastructure indicating that a Backupinfrastructure reconciliation shall be triggered.
	BackupInfrastructureReconcile = "reconcile"

	// ChartPath is the path to the Helm charts.
	ChartPath = "charts"

	// CloudConfigPrefix is a constant for the prefix which is added to secret storing the original cloud config (which
	// is being downloaded from the cloud-config-downloader process)
	CloudConfigPrefix = "cloud-config"

	// CloudConfigFilePath is the path on the shoot worker nodes to which the operating system specific configuration
	// will be downloaded.
	CloudConfigFilePath = "/var/lib/cloud-config-downloader/downloads/cloud_config"

	// CloudProviderConfigName is the name of the configmap containing the cloud provider config.
	CloudProviderConfigName = "cloud-provider-config"

	// CloudProviderConfigMapKey is the key storing the cloud provider config as value in the cloud provider configmap.
	CloudProviderConfigMapKey = "cloudprovider.conf"

	// CloudPurposeShoot is a constant used while instantiating a cloud botanist for the Shoot cluster.
	CloudPurposeShoot = "shoot"

	// CloudPurposeSeed is a constant used while instantiating a cloud botanist for the Seed cluster.
	CloudPurposeSeed = "seed"

	// ConfirmationDeletion is an annotation on a Shoot resource whose value must be set to "true" in order to
	// allow deleting the Shoot (if the annotation is not set any DELETE request will be denied).
	ConfirmationDeletion = "confirmation.garden.sapcloud.io/deletion"

	// ControllerManagerInternalConfigMapName is the name of the internal config map in which the Gardener controller
	// manager stores its configuration.
	ControllerManagerInternalConfigMapName = "gardener-controller-manager-internal-config"

	// ControllerRegistrationName is the key of a label on extension namespaces that indicates the controller registration name.
	ControllerRegistrationName = "controllerregistration.core.gardener.cloud/name"

	// DNSProviderDeprecated is the key for an annotation on a Kubernetes Secret object whose value must point to a valid
	// DNS provider.
	// deprecated
	DNSProviderDeprecated = "dns.garden.sapcloud.io/provider"

	// DNSDomainDeprecated is the key for an annotation on a Kubernetes Secret object whose value must point to a valid
	// domain name.
	// deprecated
	DNSDomainDeprecated = "dns.garden.sapcloud.io/domain"

	// DNSProvider is the key for an annotation on a Kubernetes Secret object whose value must point to a valid
	// DNS provider.
	DNSProvider = "dns.gardener.cloud/provider"

	// DNSDomain is the key for an annotation on a Kubernetes Secret object whose value must point to a valid
	// domain name.
	DNSDomain = "dns.gardener.cloud/domain"

	// EtcdRoleMain is the constant defining the role for main etcd storing data about objects in Shoot.
	EtcdRoleMain = "main"

	// EtcdMainStatefulSetName is the constant defining the statefulset name for the main etcd.
	EtcdMainStatefulSetName = "etcd-main"

	// EtcdRoleEvents is the constant defining the role for etcd storing events in Shoot.
	EtcdRoleEvents = "events"

	// EtcdEventsStatefulSetName is the constant defining the statefulset name for the events etcd.
	EtcdEventsStatefulSetName = "etcd-events"

	// GardenNamespace is the namespace in which the configuration and secrets for
	// the Gardener controller manager will be stored (e.g., secrets for the Seed clusters).
	// It is also used by the gardener-apiserver.
	GardenNamespace = "garden"

	// GardenRole is the key for an annotation on a Kubernetes object indicating what it is used for.
	GardenRole = "garden.sapcloud.io/role"

	// GardenerRole is the key for an annotation on a Kubernetes object indicating what it is used for with the new
	// naming scheme.
	GardenerRole = "gardener.cloud/role"

	// GardenRoleShoot is the value of the GardenRole key indicating type 'shoot'.
	GardenRoleShoot = "shoot"

	// GardenRoleSeed is the value of the GardenRole key indicating type 'seed'.
	GardenRoleSeed = "seed"

	// GardenRoleExtension is the value of the GardenRole key indicating type 'extension'.
	GardenRoleExtension = "extension"

	// GardenRoleControlPlane is the value of the GardenRole key indicating type 'controlplane'.
	GardenRoleControlPlane = "controlplane"

	// GardenRoleSystemComponent is the value of the GardenRole key indicating type 'system-component'.
	GardenRoleSystemComponent = "system-component"

	// GardenRoleMonitoring is the value of the GardenRole key indicating type 'monitoring'.
	GardenRoleMonitoring = "monitoring"

	// GardenRoleOptionalAddon is the value of the GardenRole key indicating type 'optional-addon'.
	GardenRoleOptionalAddon = "optional-addon"

	// GardenRoleLogging is the value of the GardenRole key indicating type 'logging'.
	GardenRoleLogging = "logging"

	// GardenRoleDefaultDomain is the value of the GardenRole key indicating type 'default-domain'.
	GardenRoleDefaultDomain = "default-domain"

	// GardenRoleInternalDomain is the value of the GardenRole key indicating type 'internal-domain'.
	GardenRoleInternalDomain = "internal-domain"

	// GardenRoleAlertingSMTP is the value of the GardenRole key indicating type 'alerting-smtp'.
	GardenRoleAlertingSMTP = "alerting-smtp"

	// GardenRoleOpenVPNDiffieHellman is the value of the GardenRole key indicating type 'openvpn-diffie-hellman'.
	GardenRoleOpenVPNDiffieHellman = "openvpn-diffie-hellman"

	// GardenRoleMembers is the value of GardenRole key indicating type 'members'.
	GardenRoleMembers = "members"

	// GardenRoleProject is the value of GardenRole key indicating type 'project'.
	GardenRoleProject = "project"

	// GardenRoleBackup is the value of GardenRole key indicating type 'backup'.
	GardenRoleBackup = "backup"

	// GardenRoleVpa is the value of GardenRole key indicating type 'vpa'.
	GardenRoleVpa = "vpa"

	// GardenCreatedBy is the key for an annotation of a Shoot cluster whose value indicates contains the username
	// of the user that created the resource.
	GardenCreatedBy = "garden.sapcloud.io/createdBy"

	// GardenOperatedBy is the key for an annotation of a Shoot cluster whose value must be a valid email address and
	// is used to send alerts to.
	GardenOperatedBy = "garden.sapcloud.io/operatedBy"

	// GardenIgnoreAlerts is the key for an annotation of a Shoot cluster whose value indicates
	// if alerts for this cluster should be ignored
	GardenIgnoreAlerts = "shoot.garden.sapcloud.io/ignore-alerts"

	// GrafanaOperatorsPrefix is a constant for a prefix used for the operators Grafana instance.
	GrafanaOperatorsPrefix = "g-operators"

	// GrafanaUsersPrefix is a constant for a prefix used for the users Grafana instance.
	GrafanaUsersPrefix = "g-users"

	// IngressPrefix is the part of a FQDN which will be used to construct the domain name for an ingress controller of
	// a Shoot cluster. For example, when a Shoot specifies domain 'cluster.example.com', the ingress domain would be
	// '*.<IngressPrefix>.cluster.example.com'.
	IngressPrefix = "ingress"

	// InternalDomainKey is a key which must be present in an internal domain constructed for a Shoot cluster. If the
	// configured internal domain already contains it, it won't be added twice. If it does not contain it, it will be
	// appended.
	InternalDomainKey = "internal"

	// KubeAPIServerDeploymentName is the name of the kube-apiserver deployment.
	KubeAPIServerDeploymentName = "kube-apiserver"

	// AWSLBReadvertiserDeploymentName is the name for the aws-lb-readvertiser
	AWSLBReadvertiserDeploymentName = "aws-lb-readvertiser"

	// KubeControllerManagerDeploymentName is the name of the kube-controller-manager deployment.
	KubeControllerManagerDeploymentName = "kube-controller-manager"

	// KubeControllerManagerServerName is the name of the kube-controller-manager server.
	KubeControllerManagerServerName = "kube-controller-manager-server"

	// MachineControllerManagerDeploymentName is the name of the machine-controller-manager deployment.
	MachineControllerManagerDeploymentName = "machine-controller-manager"

	// KubeSchedulerDeploymentName is the name of the kube-scheduler deployment.
	KubeSchedulerDeploymentName = "kube-scheduler"

	// KubeSchedulerServerName is the name of the kube-scheduler server.
	KubeSchedulerServerName = "kube-scheduler-server"

	// GardenerResourceManagerDeploymentName is the name of the gardener-resource-manager deployment.
	GardenerResourceManagerDeploymentName = "gardener-resource-manager"

	// CalicoKubeControllersDeploymentName is the name of calico-kube-controllers deployment.
	CalicoKubeControllersDeploymentName = "calico-kube-controllers"

	// CalicoTyphaDeploymentName is the name of the calico-typha deployment.
	CalicoTyphaDeploymentName = "calico-typha"

	// CoreDNSDeploymentName is the name of the coredns deployment.
	CoreDNSDeploymentName = "coredns"

	// VPNShootDeploymentName is the name of the vpn-shoot deployment.
	VPNShootDeploymentName = "vpn-shoot"

	// MetricsServerDeploymentName is the name of the metrics-server deployment.
	MetricsServerDeploymentName = "metrics-server"

	// CalicoNodeDaemonSetName is the name of the calico-node daemon set.
	CalicoNodeDaemonSetName = "calico-node"

	// KubeProxyDaemonSetName is the name of the kube-proxy daemon set.
	KubeProxyDaemonSetName = "kube-proxy"

	// GrafanaOperatorsDeploymentName is the name of the grafana deployment.
	GrafanaOperatorsDeploymentName = "grafana-operators"

	// GrafanaUsersDeploymentName is the name of the grafana deployment for the user-facing grafana.
	GrafanaUsersDeploymentName = "grafana-users"

	// KubeStateMetricsShootDeploymentName is the name of the kube-state-metrics deployment.
	KubeStateMetricsShootDeploymentName = "kube-state-metrics"

	// KubeStateMetricsSeedDeploymentName is the name of the kube-state-metrics-shoot deployment.
	KubeStateMetricsSeedDeploymentName = "kube-state-metrics-seed"

	// NodeExporterDaemonSetName is the name of the node-exporter daemon set.
	NodeExporterDaemonSetName = "node-exporter"

	// ElasticSearchStatefulSetName is the name of the elasticsearch-logging stateful set.
	ElasticSearchStatefulSetName = "elasticsearch-logging"

	// KibanaDeploymentName is the name of the kibana-logging deployment.
	KibanaDeploymentName = "kibana-logging"

	// KibanaAdminIngressCredentialsSecretName is the name of the secret which holds admin credentials.
	KibanaAdminIngressCredentialsSecretName = "logging-ingress-credentials"

	// FluentBitDaemonSetName is the name of the fluent-bit daemon set.
	FluentBitDaemonSetName = "fluent-bit"

	// FluentdEsStatefulSetName is the name of the fluentd-es stateful set.
	FluentdEsStatefulSetName = "fluentd-es"

	// ProjectPrefix is the prefix of namespaces representing projects.
	ProjectPrefix = "garden-"

	// ProjectName is they key of a label on namespaces whose value holds the project name. Usually, the label is set
	// by the Gardener Dashboard.
	ProjectName = "project.garden.sapcloud.io/name"

	// NamespaceProject is they key of a label on namespace whose value holds the project uid.
	NamespaceProject = "namespace.garden.sapcloud.io/project"

	// PrometheusStatefulSetName is the name of the Prometheus stateful set.
	PrometheusStatefulSetName = "prometheus"

	// TerraformerConfigSuffix is the suffix used for the ConfigMap which stores the Terraform configuration and variables declaration.
	TerraformerConfigSuffix = ".tf-config"

	// TerraformerVariablesSuffix is the suffix used for the Secret which stores the Terraform variables definition.
	TerraformerVariablesSuffix = ".tf-vars"

	// TerraformerStateSuffix is the suffix used for the ConfigMap which stores the Terraform state.
	TerraformerStateSuffix = ".tf-state"

	// TerraformerPodSuffix is the suffix used for the name of the Pod which validates the Terraform configuration.
	TerraformerPodSuffix = ".tf-pod"

	// TerraformerJobSuffix is the suffix used for the name of the Job which executes the Terraform configuration.
	TerraformerJobSuffix = ".tf-job"

	// TerraformerPurposeInfraDeprecated is a constant for the complete Terraform setup with purpose 'infrastructure'.
	// deprecated
	TerraformerPurposeInfraDeprecated = "infra"

	// TerraformerPurposeInternalDNSDeprecated is a constant for the complete Terraform setup with purpose 'internal cluster domain'
	// deprecated
	TerraformerPurposeInternalDNSDeprecated = "internal-dns"

	// TerraformerPurposeExternalDNSDeprecated is a constant for the complete Terraform setup with purpose 'external cluster domain'.
	// deprecated
	TerraformerPurposeExternalDNSDeprecated = "external-dns"

	// TerraformerPurposeIngressDNSDeprecated is a constant for the complete Terraform setup with purpose 'ingress domain'.
	// deprecated
	TerraformerPurposeIngressDNSDeprecated = "ingress"

	// TerraformerPurposeBackup is a constant for the complete Terraform setup with purpose 'etcd backup'.
	TerraformerPurposeBackup = "backup"

	// TerraformerPurposeKube2IAM is a constant for the complete Terraform setup with purpose 'kube2iam roles'.
	TerraformerPurposeKube2IAM = "kube2iam"

	// ShootExpirationTimestamp is an annotation on a Shoot resource whose value represents the time when the Shoot lifetime
	// is expired. The lifetime can be extended, but at most by the minimal value of the 'clusterLifetimeDays' property
	// of referenced quotas.
	ShootExpirationTimestamp = "shoot.garden.sapcloud.io/expirationTimestamp"

	// ShootNoCleanup is a constant for a label on a resource indicating the the Gardener cleaner should not delete this
	// resource when cleaning a shoot during the deletion flow.
	ShootNoCleanup = "shoot.gardener.cloud/no-cleanup"

	// ShootUseAsSeed is a constant for an annotation on a Shoot resource indicating that the Shoot shall be registered as Seed in the
	// Garden cluster once successfully created.
	ShootUseAsSeed = "shoot.garden.sapcloud.io/use-as-seed"

	// ShootStatus is a constant for a label on a Shoot resource indicating that the Shoot's health.
	// Shoot Care controller and can be used to easily identify Shoot clusters with certain states.
	ShootStatus = "shoot.garden.sapcloud.io/status"

	// ShootUnhealthy is a constant for a label on a Shoot resource indicating that the Shoot is unhealthy. It is set and unset by the
	// Shoot Care controller and can be used to easily identify Shoot clusters with issues.
	// Deprecated: Use ShootStatus instead
	ShootUnhealthy = "shoot.garden.sapcloud.io/unhealthy"

	// ShootHibernated is a constant for a label on the Shoot namespace in the Seed indicating the Shoot's hibernation status.
	// +deprecated: Use `Cluster` resource instead.
	ShootHibernated = "shoot.garden.sapcloud.io/hibernated"

	// ShootOperation is a constant for an annotation on a Shoot in a failed state indicating that an operation shall be performed.
	ShootOperation = "shoot.garden.sapcloud.io/operation"

	// ShootOperationMaintain is a constant for an annotation on a Shoot indicating that the Shoot maintenance shall be executed as soon as
	// possible.
	ShootOperationMaintain = "maintain"

	// ShootTasks is a constant for an annotation on a Shoot which states that certain tasks should be done.
	ShootTasks = "shoot.garden.sapcloud.io/tasks"

	// ShootTaskDeployInfrastructure is a name for a Shoot's infrastructure deployment task.
	ShootTaskDeployInfrastructure = "deployInfrastructure"

	// ShootTaskDeployKube2IAMResource is a name for a Shoot's Kube2IAM Resource deployment task.
	ShootTaskDeployKube2IAMResource = "deployKube2IAMResource"

	// ShootOperationRetry is a constant for an annotation on a Shoot indicating that a failed Shoot reconciliation shall be retried.
	ShootOperationRetry = "retry"

	// ShootOperationReconcile is a constant for an annotation on a Shoot indicating that a Shoot reconciliation shall be triggered.
	ShootOperationReconcile = "reconcile"

	// ShootSyncPeriod is a constant for an annotation on a Shoot which may be used to overwrite the global Shoot controller sync period.
	// The value must be a duration. It can also be used to disable the reconciliation at all by setting it to 0m. Disabling the reconciliation
	// does only mean that the period reconciliation is disabled. However, when the Gardener is restarted/redeployed or the specification is
	// changed then the reconciliation flow will be executed.
	ShootSyncPeriod = "shoot.garden.sapcloud.io/sync-period"

	// ShootIgnore is a constant for an annotation on a Shoot which may be used to tell the Gardener that the Shoot with this name should be
	// ignored completely. That means that the Shoot will never reach the reconciliation flow (independent of the operation (create/update/
	// delete)).
	ShootIgnore = "shoot.garden.sapcloud.io/ignore"

	// ShootUID is an annotation key for the shoot namespace in the seed cluster,
	// which value will be the value of `shoot.status.uid`
	ShootUID = "shoot.garden.sapcloud.io/uid"

	// AnnotateSeedNamespacePrefix is such a prefix so that the shoot namespace in the seed cluster
	// will be annotated with the annotations of the shoot resource starting with it.
	// For example, if the shoot is annotated with <AnnotateSeedNamespacePrefix>key=value,
	// then the namespace in the seed will be annotated with <AnnotateSeedNamespacePrefix>key=value, as well.
	AnnotateSeedNamespacePrefix = "custom.shoot.sapcloud.io/"

	// AnnotatePersistentVolumeMinimumSize is used to specify the minimum size of persistent volume in the cluster
	AnnotatePersistentVolumeMinimumSize = "persistentvolume.garden.sapcloud.io/minimumSize"

	// AnnotatePersistentVolumeProvider is used to tell volume provider in the k8s cluster
	AnnotatePersistentVolumeProvider = "persistentvolume.garden.sapcloud.io/provider"

	// BackupNamespacePrefix is a constant for backup namespace created for shoot's backup infrastructure related resources.
	BackupNamespacePrefix = "backup"

	// GardenerResourceManagerImageName is the name of the GardenerResourceManager image.
	GardenerResourceManagerImageName = "gardener-resource-manager"

	// CalicoNodeImageName is the name of the CalicoNode image.
	CalicoNodeImageName = "calico-node"

	// CalicoCNIImageName is the name of the CalicoCNI image.
	CalicoCNIImageName = "calico-cni"

	// CalicoTyphaImageName is the name of the CalicoTypha image.
	CalicoTyphaImageName = "calico-typha"

	// CalicoKubeControllersImageName is the name of the CalicoKubeControllers image.
	CalicoKubeControllersImageName = "calico-kube-controllers"

	// CoreDNSImageName is the name of the CoreDNS image.
	CoreDNSImageName = "coredns"

	// HyperkubeImageName is the name of the Hyperkube image.
	HyperkubeImageName = "hyperkube"

	// MetricsServerImageName is the name of the MetricsServer image.
	MetricsServerImageName = "metrics-server"

	// VPNShootImageName is the name of the VPNShoot image.
	VPNShootImageName = "vpn-shoot"

	// VPNSeedImageName is the name of the VPNSeed image.
	VPNSeedImageName = "vpn-seed"

	// NodeExporterImageName is the name of the NodeExporter image.
	NodeExporterImageName = "node-exporter"

	// KubeLegoImageName is the name of the KubeLego image.
	KubeLegoImageName = "kube-lego"

	// Kube2IAMImageName is the name of the Kube2IAM image.
	Kube2IAMImageName = "kube2iam"

	// KubernetesDashboardImageName is the name of the KubernetesDashboard image.
	KubernetesDashboardImageName = "kubernetes-dashboard"

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

	// ETCDImageName is the name of the ETCD image.
	ETCDImageName = "etcd"

	// CSINodeDriverRegistrarImageName is the name of driver registrar - https://github.com/kubernetes-csi/node-driver-registrar
	CSINodeDriverRegistrarImageName = "csi-node-driver-registrar"

	// CSIPluginAlicloudImageName is the name of csi plugin for Alicloud - https://github.com/AliyunContainerService/csi-plugin
	CSIPluginAlicloudImageName = "csi-plugin-alicloud"

	// CSIPluginPacketImageName is the name of csi plugin for Packet - https://github.com/packethost/csi-packet
	CSIPluginPacketImageName = "packet-storage-interface"

	// AWSLBReadvertiserImageName is the name of the AWSLBReadvertiser image.
	AWSLBReadvertiserImageName = "aws-lb-readvertiser"

	// PauseContainerImageName is the name of the PauseContainer image.
	PauseContainerImageName = "pause-container"

	// TerraformerImageName is the name of the Terraformer image.
	TerraformerImageName = "terraformer"

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

	// DependencyWatchdogDeploymentName is the name of the dependency controller resources.
	DependencyWatchdogDeploymentName = "dependency-watchdog"

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
)

var (
	// TerraformerChartPath is the path where the seed-terraformer charts reside.
	TerraformerChartPath = filepath.Join(ChartPath, "seed-terraformer", "charts")

	// ETCDMainStatefulSetName is the name of the etcd-main stateful set.
	ETCDMainStatefulSetName = fmt.Sprintf("etcd-%s", EtcdRoleMain)
	// ETCDEventsStatefulSetName is the name of the etcd-events stateful set.
	ETCDEventsStatefulSetName = fmt.Sprintf("etcd-%s", EtcdRoleEvents)

	// RequiredControlPlaneDeployments is a set of the required shoot control plane deployments
	// running in the seed.
	RequiredControlPlaneDeployments = sets.NewString(
		GardenerResourceManagerDeploymentName,
		KubeAPIServerDeploymentName,
		KubeControllerManagerDeploymentName,
		KubeSchedulerDeploymentName,
		MachineControllerManagerDeploymentName,
		DependencyWatchdogDeploymentName,
	)

	// RequiredControlPlaneStatefulSets is a set of the required shoot control plane stateful
	// sets running in the seed.
	RequiredControlPlaneStatefulSets = sets.NewString(
		ETCDMainStatefulSetName,
		ETCDEventsStatefulSetName,
	)

	// RequiredSystemComponentDeployments is a set of the required system components.
	RequiredSystemComponentDeployments = sets.NewString(
		CalicoKubeControllersDeploymentName,
		CalicoTyphaDeploymentName,
		CoreDNSDeploymentName,
		VPNShootDeploymentName,
		MetricsServerDeploymentName,
	)

	// RequiredSystemComponentDaemonSets is a set of the required shoot control plane daemon sets.
	RequiredSystemComponentDaemonSets = sets.NewString(
		CalicoNodeDaemonSetName,
		KubeProxyDaemonSetName,
	)

	// RequiredMonitoringSeedDeployments is a set of the required seed monitoring deployments.
	RequiredMonitoringSeedDeployments = sets.NewString(
		GrafanaOperatorsDeploymentName,
		GrafanaUsersDeploymentName,
		KubeStateMetricsSeedDeploymentName,
		KubeStateMetricsShootDeploymentName,
	)

	// RequiredMonitoringShootDaemonSets is a set of the required shoot monitoring daemon sets.
	RequiredMonitoringShootDaemonSets = sets.NewString(
		NodeExporterDaemonSetName,
	)

	// RequiredLoggingStatefulSets is a set of the required logging stateful sets.
	RequiredLoggingStatefulSets = sets.NewString(
		ElasticSearchStatefulSetName,
	)

	// RequiredLoggingDeployments is a set of the required logging deployments.
	RequiredLoggingDeployments = sets.NewString(
		KibanaDeploymentName,
	)
)
