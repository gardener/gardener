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

	// BackupSecretName defines the name of the secret containing the credentials which are required to
	// authenticate against the respective cloud provider (required to store the backups of Shoot clusters).
	BackupSecretName = "etcd-backup"

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

	// CloudProviderSecretName is the name of the secret containing the cloud provider credentials.
	CloudProviderSecretName = "cloudprovider"

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

	// DNSProvider is the key for an annotation on a Kubernetes Secret object whose value must point to a valid
	// DNS provider.
	DNSProvider = "dns.garden.sapcloud.io/provider"

	// DNSDomain is the key for an annotation on a Kubernetes Secret object whose value must point to a valid
	// domain name.
	DNSDomain = "dns.garden.sapcloud.io/domain"

	// DNSHostedZoneID is the key for an annotation on a Kubernetes Secret object whose value must point to a valid
	// DNS Hosted Zone.
	DNSHostedZoneID = "dns.garden.sapcloud.io/hostedZoneID"

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

	// GardenRoleShoot is the value of the GardenRole key indicating type 'shoot'.
	GardenRoleShoot = "shoot"

	// GardenRoleSeed is the value of the GardenRole key indicating type 'seed'.
	GardenRoleSeed = "seed"

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

	// GardenRoleMembers ist the value of GardenRole key indicating type 'members'.
	GardenRoleMembers = "members"

	//GardenRoleProject is the value of GardenRole key indicating type 'project'.
	GardenRoleProject = "project"

	//GardenRoleBackup is the value of GardenRole key indicating type 'backup'.
	GardenRoleBackup = "backup"

	// GardenRoleCertificateManagement is the value of GardenRole key indicating type 'certificate-management'.
	GardenRoleCertificateManagement = "certificate-management"

	// GardenRoleVpa is the value of GardenRole key indecating type 'vpa'.
	GardenRoleVpa = "vpa"

	// GardenCreatedBy is the key for an annotation of a Shoot cluster whose value indicates contains the username
	// of the user that created the resource.
	GardenCreatedBy = "garden.sapcloud.io/createdBy"

	// GardenOperatedBy is the key for an annotation of a Shoot cluster whose value must be a valid email address and
	// is used to send alerts to.
	GardenOperatedBy = "garden.sapcloud.io/operatedBy"

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

	// EnableHPANodeCount is the number of nodes in shoot cluster after which HPA is deployed to autoscale kube-apiserver.
	EnableHPANodeCount = 5

	// CloudControllerManagerDeploymentName is the name of the cloud-controller-manager deployment.
	CloudControllerManagerDeploymentName = "cloud-controller-manager"

	// CloudControllerManagerServerName is the name of the cloud-controller-manager server.
	CloudControllerManagerServerName = "cloud-controller-manager-server"

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

	// KubeAddonManagerDeploymentName is the name of the kube-addon-manager deployment.
	KubeAddonManagerDeploymentName = "kube-addon-manager"

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

	// GrafanaDeploymentName is the name of the grafana deployment.
	GrafanaDeploymentName = "grafana"

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

	// FluentBitDaemonSetName is the name of the fluent-bit daemon set.
	FluentBitDaemonSetName = "fluent-bit"

	// FluentdEsStatefulSetName is the name of the fluentd-es stateful set.
	FluentdEsStatefulSetName = "fluentd-es"

	// ProjectPrefix is the prefix of namespaces representing projects.
	ProjectPrefix = "garden-"

	// ProjectName is they key of a label on namespaces whose value holds the project name. Usually, the label is set
	// by the Gardener Dashboard.
	ProjectName = "project.garden.sapcloud.io/name"

	// ProjectNamespace is they key of a label on projects whose value holds the namespace name. Usually, the label is set
	// by the Gardener Dashboard.
	ProjectNamespace = "project.garden.sapcloud.io/namespace"

	// NamespaceProject is they key of a label on namespace whose value holds the project uid.
	NamespaceProject = "namespace.garden.sapcloud.io/project"

	// ProjectOwner is they key of a label on namespaces whose value holds the project owner. Usually, the label is set
	// by the Gardener Dashboard.
	ProjectOwner = "project.garden.sapcloud.io/owner"

	// ProjectDescription is they key of a label on namespaces whose value holds the project description. Usually, the label is set
	// by the Gardener Dashboard.
	ProjectDescription = "project.garden.sapcloud.io/description"

	// ProjectPurpose is they key of a label on namespaces whose value holds the project purpose. Usually, the label is set
	// by the Gardener Dashboard.
	ProjectPurpose = "project.garden.sapcloud.io/purpose"

	// ProjectMemberClusterRole is the name of the cluster role defining the permissions for project members.
	ProjectMemberClusterRole = "garden.sapcloud.io:system:project-member"

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

	// TerraformerPurposeInfra is a constant for the complete Terraform setup with purpose 'infrastructure'.
	TerraformerPurposeInfra = "infra"

	// TerraformerPurposeInternalDNS is a constant for the complete Terraform setup with purpose 'internal cluster domain'
	TerraformerPurposeInternalDNS = "internal-dns"

	// TerraformerPurposeExternalDNS is a constant for the complete Terraform setup with purpose 'external cluster domain'.
	TerraformerPurposeExternalDNS = "external-dns"

	// TerraformerPurposeBackup is a constant for the complete Terraform setup with purpose 'etcd backup'.
	TerraformerPurposeBackup = "backup"

	// TerraformerPurposeKube2IAM is a constant for the complete Terraform setup with purpose 'kube2iam roles'.
	TerraformerPurposeKube2IAM = "kube2iam"

	// TerraformerPurposeIngress is a constant for the complete Terraform setup with purpose 'ingress'.
	TerraformerPurposeIngress = "ingress"

	// ShootExpirationTimestamp is an annotation on a Shoot resource whose value represents the time when the Shoot lifetime
	// is expired. The lifetime can be extended, but at most by the minimal value of the 'clusterLifetimeDays' property
	// of referenced quotas.
	ShootExpirationTimestamp = "shoot.garden.sapcloud.io/expirationTimestamp"

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

	//AnnotatePersistentVolumeMinimumSize is used to specify the minimum size of persistent volume in the cluster
	AnnotatePersistentVolumeMinimumSize = "persistentvolume.garden.sapcloud.io/minimumSize"

	// BackupNamespacePrefix is a constant for backup namespace created for shoot's backup infrastructure related resources.
	BackupNamespacePrefix = "backup"

	// KubeAddonManagerImageName is the name of the KubeAddonManager image.
	KubeAddonManagerImageName = "kube-addon-manager"

	// CalicoNodeImageName is the name of the CalicoNode image.
	CalicoNodeImageName = "calico-node"

	// CalicoCNIImageName is the name of the CalicoCNI image.
	CalicoCNIImageName = "calico-cni"

	// CalicoTyphaImageName is the name of the CalicoTypha image.
	CalicoTyphaImageName = "calico-typha"

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

	// MachineControllerManagerImageName is the name of the MachineControllerManager image.
	MachineControllerManagerImageName = "machine-controller-manager"

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

	// ETCDBackupRestoreImageName is the name of the ETCDBackupRestore image.
	ETCDBackupRestoreImageName = "etcd-backup-restore"

	// AlicloudControllerManagerImageName is the name of the AlicloudControllerManager image.
	AlicloudControllerManagerImageName = "alicloud-controller-manager"

	// CSI Images

	// CSIAttacherImageName is the name of csi attacher - https://github.com/kubernetes-csi/external-attacher
	CSIAttacherImageName = "csi-attacher"

	// CSIDriverRegistrarImageName is the name of driver registrar - https://github.com/kubernetes-csi/driver-registrar
	CSIDriverRegistrarImageName = "csi-driver-registrar"

	// CSINodeDriverRegistrarImageName is the name of driver registrar - https://github.com/kubernetes-csi/node-driver-registrar
	CSINodeDriverRegistrarImageName = "csi-node-driver-registrar"

	// CSIProvisionerImageName is the name of csi provisioner - https://github.com/kubernetes-csi/external-provisioner
	CSIProvisionerImageName = "csi-provisioner"

	// CSISnapshotterImageName is the name of csi plugin for Alicloud - https://github.com/kubernetes-csi/external-snapshotter
	CSISnapshotterImageName = "csi-snapshotter"

	// CSIPluginAlicloudImageName is the name of csi plugin for Alicloud - https://github.com/AliyunContainerService/csi-plugin
	CSIPluginAlicloudImageName = "csi-plugin-alicloud"

	// AWSLBReadvertiserImageName is the name of the AWSLBReadvertiser image.
	AWSLBReadvertiserImageName = "aws-lb-readvertiser"

	// PauseContainerImageName is the name of the PauseContainer image.
	PauseContainerImageName = "pause-container"

	// TerraformerImageName is the name of the Terraformer image.
	TerraformerImageName = "terraformer"

	// ElasticsearchImageName is the name of the Elastic-Search image used for logging
	ElasticsearchImageName = "elasticsearch-oss"

	// CuratorImageName is the name of the curator image used to alter the Elastic-search logs
	CuratorImageName = "curator-es"

	// KibanaImageName is the name of the Kibana image used for logging  UI
	KibanaImageName = "kibana-oss"

	// FluentdEsImageName is the image of the Fluentd image used for logging
	FluentdEsImageName = "fluentd-es"

	// FluentBitImageName is the image of Fluent-bit image
	FluentBitImageName = "fluent-bit"

	// AlpineImageName is the name of alpine image
	AlpineImageName = "alpine"

	// CertManagerImageName is the name of cert-manager image
	CertManagerImageName = "cert-manager"

	// CertManagerResourceName is the name of the Cert-Manager resources.
	CertManagerResourceName = "cert-manager"

	// CertBrokerImageName is the name of cert-broker image.
	CertBrokerImageName = "cert-broker"

	// CertBrokerResourceName is the name of the Cert-Broker resources.
	CertBrokerResourceName = "cert-broker"

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
		CloudControllerManagerDeploymentName,
		KubeAddonManagerDeploymentName,
		KubeAPIServerDeploymentName,
		KubeControllerManagerDeploymentName,
		KubeSchedulerDeploymentName,
		MachineControllerManagerDeploymentName,
	)

	// RequiredControlPlaneStatefulSets is a set of the required shoot control plane stateful
	// sets running in the seed.
	RequiredControlPlaneStatefulSets = sets.NewString(
		ETCDMainStatefulSetName,
		ETCDEventsStatefulSetName,
	)

	// RequiredSystemComponentDeployments is a set of the required system components.
	RequiredSystemComponentDeployments = sets.NewString(
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
		GrafanaDeploymentName,
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

// CloudConfigUserDataConfig is a struct containing cloud-specific configuration required to
// render the shoot-cloud-config chart properly.
type CloudConfigUserDataConfig struct {
	ProvisionCloudProviderConfig bool
	KubeletParameters            []string
	HostnameOverride             bool
	EnableCSI                    bool
	ProviderIDProvided           bool
}
