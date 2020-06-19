// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package constants

const (
	// SecretNameCACluster is a constant for the name of a Kubernetes secret object that contains the CA
	// certificate of a shoot cluster.
	SecretNameCACluster = "ca"
	// SecretNameCAETCD is a constant for the name of a Kubernetes secret object that contains the CA
	// certificate of the etcd of a shoot cluster.
	SecretNameCAETCD = "ca-etcd"
	// SecretNameCAFrontProxy is a constant for the name of a Kubernetes secret object that contains the CA
	// certificate of the kube-aggregator a shoot cluster.
	SecretNameCAFrontProxy = "ca-front-proxy"
	// SecretNameCAKubelet is a constant for the name of a Kubernetes secret object that contains the CA
	// certificate of the kubelet of a shoot cluster.
	SecretNameCAKubelet = "ca-kubelet"
	// SecretNameCAMetricsServer is a constant for the name of a Kubernetes secret object that contains the CA
	// certificate of the metrics-server of a shoot cluster.
	SecretNameCAMetricsServer = "ca-metrics-server"
	// SecretNameCloudProvider is a constant for the name of a Kubernetes secret object that contains the provider
	// specific credentials that shall be used to create/delete the shoot.
	SecretNameCloudProvider = "cloudprovider"
	// SecretNameSSHKeyPair is a constant for the name of a Kubernetes secret object that contains the SSH key pair
	// (public and private key) that can be used to SSH into the shoot nodes.
	SecretNameSSHKeyPair = "ssh-keypair"

	// SecretNameGardener is a constant for the name of a Kubernetes secret object that contains the client
	// certificate and a kubeconfig for a shoot cluster. It is used by Gardener and can be used by extension
	// controllers in order to communicate with the shoot's API server. The client certificate has administrator
	// privileges.
	SecretNameGardener = "gardener"
	// SecretNameGardenerInternal is a constant for the name of a Kubernetes secret object that contains the client
	// certificate and a kubeconfig for a shoot cluster. It is used by Gardener and can be used by extension
	// controllers in order to communicate with the shoot's API server. The client certificate has administrator
	// privileges. The difference to the "gardener" secret is that is contains the in-cluster endpoint as address to
	// for the shoot API server instead the DNS name or load balancer address.
	SecretNameGardenerInternal = "gardener-internal"

	// DeploymentNameClusterAutoscaler is a constant for the name of a Kubernetes deployment object that contains
	// the cluster-autoscaler pod.
	DeploymentNameClusterAutoscaler = "cluster-autoscaler"
	// DeploymentNameKubeAPIServer is a constant for the name of a Kubernetes deployment object that contains
	// the kube-apiserver pod.
	DeploymentNameKubeAPIServer = "kube-apiserver"
	// DeploymentNameKubeControllerManager is a constant for the name of a Kubernetes deployment object that contains
	// the kube-controller-manager pod.
	DeploymentNameKubeControllerManager = "kube-controller-manager"

	// DeploymentNameKubeScheduler is a constant for the name of a Kubernetes deployment object that contains
	// the kube-scheduler pod.
	DeploymentNameKubeScheduler = "kube-scheduler"
	// DeploymentNameGardenerResourceManager is a constant for the name of a Kubernetes deployment object that contains
	// the gardener-resource-manager pod.
	DeploymentNameGardenerResourceManager = "gardener-resource-manager"
	// DeploymentNameGrafanaOperators is a constant for the name of a Kubernetes deployment object that contains
	// the grafana-operators pod.
	DeploymentNameGrafanaOperators = "grafana-operators"
	// DeploymentNameGrafanaUsers is a constant for the name of a Kubernetes deployment object that contains
	// the grafana-users pod.
	DeploymentNameGrafanaUsers = "grafana-users"
	// DeploymentNameKubeStateMetricsShoot is a constant for the name of a Kubernetes deployment object that contains
	// the kube-state-metrics pod.
	DeploymentNameKubeStateMetricsShoot = "kube-state-metrics"
	// DeploymentNameKubeStateMetricsSeed is a constant for the name of a Kubernetes deployment object that contains
	// the kube-state-metrics-seed pod.
	DeploymentNameKubeStateMetricsSeed = "kube-state-metrics-seed"
	// DeploymentNameKibana is a constant for the name of a Kubernetes deployment object that contains
	// the kibana-logging pod.
	DeploymentNameKibana = "kibana-logging"

	// StatefulSetNameAlertManager is a constant for the name of a Kubernetes stateful set object that contains
	// the alertmanager pod.
	StatefulSetNameAlertManager = "alertmanager"
	// ETCDMain is a constant for the name of etcd-main Etcd object.
	ETCDMain = "etcd-main"
	// ETCDEvents is a constant for the name of etcd-events Etcd object.
	ETCDEvents = "etcd-events"
	// StatefulSetNameElasticSearch is a constant for the name of a Kubernetes stateful set object that contains
	// the elasticsearch-logging pod.
	StatefulSetNameElasticSearch = "elasticsearch-logging"
	// StatefulSetNamePrometheus is a constant for the name of a Kubernetes stateful set object that contains
	// the prometheus pod.
	StatefulSetNamePrometheus = "prometheus"

	// GardenerPurpose is a constant for the key in a label describing the purpose of the respective object.
	GardenerPurpose = "gardener.cloud/purpose"

	// GardenerOperation is a constant for an annotation on a resource that describes a desired operation.
	GardenerOperation = "gardener.cloud/operation"
	// GardenerOperationReconcile is a constant for the value of the operation annotation describing a reconcile
	// operation.
	GardenerOperationReconcile = "reconcile"
	// GardenerTimestamp is a constant for an annotation on a resource that describes the timestamp when a reconciliation has been requested.
	// It is only used to guarantee an update event for watching clients in case the operation-annotation is already present.
	GardenerTimestamp = "gardener.cloud/timestamp"
	// GardenerOperationMigrate is a constant for the value of the operation annotation describing a migration
	// operation.
	GardenerOperationMigrate = "migrate"
	// GardenerOperationRestore is a constant for the value of the operation annotation describing a restoration
	// operation.
	GardenerOperationRestore = "restore"
	// GardenerOperationWaitForState is a constant for the value of the operation annotation describing a wait
	// operation.
	GardenerOperationWaitForState = "wait-for-state"

	// DeprecatedGardenRole is the key for an annotation on a Kubernetes object indicating what it is used for.
	//
	// Deprecated: Use `GardenRole` instead.
	DeprecatedGardenRole = "garden.sapcloud.io/role"
	// GardenRole is a constant for a label that describes a role.
	GardenRole = "gardener.cloud/role"
	// GardenRoleExtension is a constant for a label that describes the 'extensions' role.
	GardenRoleExtension = "extension"
	// GardenRoleSeed is the value of the GardenRole key indicating type 'seed'.
	GardenRoleSeed = "seed"
	// GardenRoleShoot is the value of the GardenRole key indicating type 'shoot'.
	GardenRoleShoot = "shoot"
	// GardenRoleLogging is the value of the GardenRole key indicating type 'logging'.
	GardenRoleLogging = "logging"
	// GardenRoleProject is the value of GardenRole key indicating type 'project'.
	GardenRoleProject = "project"
	// GardenRoleControlPlane is the value of the GardenRole key indicating type 'controlplane'.
	GardenRoleControlPlane = "controlplane"
	// GardenRoleSystemComponent is the value of the GardenRole key indicating type 'system-component'.
	GardenRoleSystemComponent = "system-component"
	// GardenRoleMonitoring is the value of the GardenRole key indicating type 'monitoring'.
	GardenRoleMonitoring = "monitoring"
	// GardenRoleOptionalAddon is the value of the GardenRole key indicating type 'optional-addon'.
	GardenRoleOptionalAddon = "optional-addon"

	// DeprecatedShootUID is an annotation key for the shoot namespace in the seed cluster,
	// which value will be the value of `shoot.status.uid`
	// +deprecated: Use `Cluster` resource instead.
	DeprecatedShootUID = "shoot.garden.sapcloud.io/uid"

	// SeedResourceManagerClass is the resource-class managed by the Gardener-Resource-Manager
	// instance in the garden namespace on the seeds.
	SeedResourceManagerClass = "seed"
	// LabelBackupProvider is used to identify the backup provider.
	LabelBackupProvider = "backup.gardener.cloud/provider"
	// LabelSeedProvider is used to identify the seed provider.
	LabelSeedProvider = "seed.gardener.cloud/provider"
	// LabelShootProvider is used to identify the shoot provider.
	LabelShootProvider = "shoot.gardener.cloud/provider"
	// LabelNetworkingProvider is used to identify the networking provider for the cni plugin.
	LabelNetworkingProvider = "networking.shoot.gardener.cloud/provider"
	// LabelExtensionConfiguration is used to identify the provider's configuration which will be added to Gardener configuration
	LabelExtensionConfiguration = "extensions.gardener.cloud/configuration"
	// LabelLogging is a constant for a label for logging stack configurations
	LabelLogging = "logging"
	// LabelMonitoring is a constant for a label for monitoring stack configurations
	LabelMonitoring = "monitoring"

	// LabelNetworkPolicyToBlockedCIDRs allows Egress from pods labeled with 'networking.gardener.cloud/to-blocked-cidrs=allowed'.
	LabelNetworkPolicyToBlockedCIDRs = "networking.gardener.cloud/to-blocked-cidrs"
	// LabelNetworkPolicyToDNS allows Egress from pods labeled with 'networking.gardener.cloud/to-dns=allowed' to DNS running in 'kube-system'.
	// In practice, most of the Pods which require network Egress need this label.
	LabelNetworkPolicyToDNS = "networking.gardener.cloud/to-dns"
	// LabelNetworkPolicyToPrivateNetworks allows Egress from pods labeled with 'networking.gardener.cloud/to-private-networks=allowed' to the
	// private networks (RFC1918), Carrier-grade NAT (RFC6598) except for cloudProvider's specific metadata service IP, seed networks,
	// shoot networks.
	LabelNetworkPolicyToPrivateNetworks = "networking.gardener.cloud/to-private-networks"
	// LabelNetworkPolicyToPublicNetworks allows Egress from pods labeled with 'networking.gardener.cloud/to-public-networks=allowed' to all public
	// network IPs, except for private networks (RFC1918), carrier-grade NAT (RFC6598), cloudProvider's specific metadata service IP.
	// In practice, this blocks Egress traffic to all networks in the Seed cluster and only traffic to public IPv4 addresses.
	LabelNetworkPolicyToPublicNetworks = "networking.gardener.cloud/to-public-networks"
	// LabelNetworkPolicyToSeedAPIServer allows Egress from pods labeled with 'networking.gardener.cloud/to-seed-apiserver=allowed' to Seed's Kubernetes
	// API Server.
	LabelNetworkPolicyToSeedAPIServer = "networking.gardener.cloud/to-seed-apiserver"
	// LabelNetworkPolicyToShootAPIServer allows Egress from pods labeled with 'networking.gardener.cloud/to-shoot-apiserver=allowed' to talk to Shoot's
	// Kubernetes API Server.
	LabelNetworkPolicyToShootAPIServer = "networking.gardener.cloud/to-shoot-apiserver"
	// LabelNetworkPolicyFromShootAPIServer allows Egress from Shoot's Kubernetes API Server to talk to pods labeled with
	// 'networking.gardener.cloud/from-shoot-apiserver=allowed'.
	LabelNetworkPolicyFromShootAPIServer = "networking.gardener.cloud/from-shoot-apiserver"
	// LabelNetworkPolicyToAll disables all Ingress and Egress traffic into/from this namespace when set to "disallowed".
	LabelNetworkPolicyToAll = "networking.gardener.cloud/to-all"
	// LabelNetworkPolicyToElasticSearch allows Ingress to the ElasticSearch API pods labeled with 'networking.gardener.cloud/to-elasticsearch=allowed',
	// and fluentd in 'garden' namespace.
	LabelNetworkPolicyToElasticSearch = "networking.gardener.cloud/to-elasticsearch"
	// LabelNetworkPolicyFromPrometheus allows Ingress from Prometheus to pods labeled with 'networking.gardener.cloud/from-prometheus=allowed' and ports
	// named 'metrics' in the PodSpecification.
	LabelNetworkPolicyFromPrometheus = "networking.gardener.cloud/from-prometheus"
	// LabelNetworkPolicyAllowed is a constant for allowing a network policy.
	LabelNetworkPolicyAllowed = "allowed"
	// LabelNetworkPolicyDisallowed is a constant for disallowing a network policy.
	LabelNetworkPolicyDisallowed = "disallowed"

	// LabelApp is a constant for a label key.
	LabelApp = "app"
	// LabelRole is a constant for a label key.
	LabelRole = "role"
	// LabelKubernetes is a constant for a label for Kubernetes workload.
	LabelKubernetes = "kubernetes"
	// LabelAPIServer is a constant for a label for the kube-apiserver.
	LabelAPIServer = "apiserver"
	// LabelControllerManager is a constant for a label for the kube-controller-manager.
	LabelControllerManager = "controller-manager"
	// LabelScheduler is a constant for a label for the kube-scheduler.
	LabelScheduler = "scheduler"
	// LabelExtensionProjectRole is a constant for a label value for extension project roles
	LabelExtensionProjectRole = "extension-project-role"

	// LabelAPIServerExposure is a constant for label key which gardener can add to various objects related
	// to kube-apiserver exposure.
	LabelAPIServerExposure = "core.gardener.cloud/apiserver-exposure"
	// LabelAPIServerExposureGardenerManaged is a constant for label value which gardener sets on the label key
	// "core.gardener.cloud/apiserver-exposure" to indicate that it's responsible for apiserver exposure (via SNI).
	LabelAPIServerExposureGardenerManaged = "gardener-managed"

	// GardenNamespace is the namespace in which the configuration and secrets for
	// the Gardener controller manager will be stored (e.g., secrets for the Seed clusters).
	// It is also used by the gardener-apiserver.
	GardenNamespace = "garden"

	// AnnotationShootUseAsSeed is a constant for an annotation on a Shoot resource indicating that the Shoot shall be registered as Seed in the
	// Garden cluster once successfully created.
	AnnotationShootUseAsSeed = "shoot.gardener.cloud/use-as-seed"
	// AnnotationShootUseAsSeedDeprecated is a constant for an annotation on a Shoot resource indicating that the Shoot shall be registered as Seed in the
	// Garden cluster once successfully created.
	//
	// Deprecated: Use `AnnotationShootUseAsSeed` instead.
	AnnotationShootUseAsSeedDeprecated = "shoot.garden.sapcloud.io/use-as-seed"
	// AnnotationShootIgnoreAlerts is the key for an annotation of a Shoot cluster whose value indicates
	// if alerts for this cluster should be ignored
	AnnotationShootIgnoreAlerts = "shoot.gardener.cloud/ignore-alerts"
	// AnnotationShootIgnoreAlertsDeprecated is the key for an annotation of a Shoot cluster whose value indicates
	// if alerts for this cluster should be ignored
	//
	// Deprecated: Use `AnnotationShootIgnoreAlerts` instead.
	AnnotationShootIgnoreAlertsDeprecated = "shoot.garden.sapcloud.io/ignore-alerts"
	// AnnotationShootSkipCleanup is a key for an annotation on a Shoot resource that declares that the clean up steps should be skipped when the
	// cluster is deleted. Concretely, this will skip everything except the deletion of (load balancer) services and persistent volume resources.
	AnnotationShootSkipCleanup = "shoot.gardener.cloud/skip-cleanup"

	// OperatingSystemConfigUnitNameKubeletService is a constant for a unit in the operating system config that contains the kubelet service.
	OperatingSystemConfigUnitNameKubeletService = "kubelet.service"
	// OperatingSystemConfigUnitNameDockerService is a constant for a unit in the operating system config that contains the docker service.
	OperatingSystemConfigUnitNameDockerService = "docker.service"
	// OperatingSystemConfigUnitNameContainerDService is a constant for a unit in the operating system config that contains the containerd service.
	OperatingSystemConfigUnitNameContainerDService = "containerd.service"
	// OperatingSystemConfigFilePathKernelSettings is a constant for a path to a file in the operating system config that contains some general kernel settings.
	OperatingSystemConfigFilePathKernelSettings = "/etc/sysctl.d/99-k8s-general.conf"
	// OperatingSystemConfigFilePathKubeletConfig is a constant for a path to a file in the operating system config that contains the kubelet configuration.
	OperatingSystemConfigFilePathKubeletConfig = "/var/lib/kubelet/config/kubelet"

	// FluentBitConfigMapKubernetesFilter is a constant for the Fluent Bit ConfigMap's section regarding Kubernetes filters
	FluentBitConfigMapKubernetesFilter = "filter-kubernetes.conf"
	// FluentBitConfigMapParser is a constant for the Fluent Bit ConfigMap's section regarding Parsers for common container types
	FluentBitConfigMapParser = "parsers.conf"
	// PrometheusConfigMapAlertingRules is a constant for the Prometheus alerting rules tag in provider-specific monitoring configuration
	PrometheusConfigMapAlertingRules = "alerting_rules"
	// PrometheusConfigMapScrapeConfig is a constant for the Prometheus scrape config tag in provider-specific monitoring configuration
	PrometheusConfigMapScrapeConfig = "scrape_config"
	// GrafanaConfigMapUserDashboard is a constant for the Grafana user dashboard tag in provider-specific monitoring configuration
	GrafanaConfigMapUserDashboard = "dashboard_users"
	// GrafanaConfigMapOperatorDashboard is a constant for the Grafana operator dashboard tag in provider-specific monitoring configuration
	GrafanaConfigMapOperatorDashboard = "dashboard_operators"

	// LabelControllerRegistrationName is the key of a label on extension namespaces that indicates the controller registration name.
	LabelControllerRegistrationName = "controllerregistration.core.gardener.cloud/name"

	// EventResourceReferenced indicates that the resource deletion is in waiting mode because the resource is still
	// being referenced by at least one other resource (e.g. a SecretBinding is still referenced by a Shoot)
	EventResourceReferenced = "ResourceReferenced"

	// LabelPodMaintenanceRestart is a constant for a label that describes that a pod should be restarted during maintenance.
	LabelPodMaintenanceRestart = "maintenance.gardener.cloud/restart"

	// LabelWorkerPool is a constant for a label that indicates the worker pool the node belongs to
	LabelWorkerPool = "worker.gardener.cloud/pool"
	// LabelWorkerPool is a deprecated constant for a label that indicates the worker pool the node belongs to
	LabelWorkerPoolDeprecated = "worker.garden.sapcloud.io/group"

	// LabelWorkerPoolSystemComponents is a constant that indicates whether the worker pool should host system components
	LabelWorkerPoolSystemComponents = "worker.gardener.cloud/system-components"

	// ReferencedResourcesPrefix is the prefix used when copying referenced resources to the Shoot namespace in the Seed,
	// to avoid naming collisions with resources managed by Gardener.
	ReferencedResourcesPrefix = "ref-"
)
