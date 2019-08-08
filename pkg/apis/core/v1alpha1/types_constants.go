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

package v1alpha1

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

	// DeploymentNameClusterAutoscaler is a constant for the name of a Kubernetes deployment object that contains
	// the cluster-autoscaler pod.
	DeploymentNameClusterAutoscaler = "cluster-autoscaler"
	// DeploymentNameClusterAutoscaler is a constant for the name of a Kubernetes deployment object that contains
	// the dependency-watchdog pod.
	DeploymentNameDependencyWatchdog = "dependency-watchdog"
	// DeploymentNameClusterAutoscaler is a constant for the name of a Kubernetes deployment object that contains
	// the kube-apiserver pod.
	DeploymentNameKubeAPIServer = "kube-apiserver"
	// DeploymentNameClusterAutoscaler is a constant for the name of a Kubernetes deployment object that contains
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

	// StatefulSetNameETCDMain is a constant for the name of a Kubernetes stateful set object that contains
	// the etcd-main pod.
	StatefulSetNameETCDMain = "etcd-main"
	// StatefulSetNameETCDEvents is a constant for the name of a Kubernetes stateful set object that contains
	// the etcd-events pod.
	StatefulSetNameETCDEvents = "etcd-events"
	// StatefulSetNameElasticSearch is a constant for the name of a Kubernetes stateful set object that contains
	// the elasticsearch-logging pod.
	StatefulSetNameElasticSearch = "elasticsearch-logging"
	// StatefulSetNamePrometheus is a constant for the name of a Kubernetes stateful set object that contains
	// the prometheus pod.
	StatefulSetNamePrometheus = "prometheus"

	// GardenPurpose is a constant for the key in a label describing the purpose of the respective object.
	GardenPurpose = "garden.sapcloud.io/purpose"
	// GardenerPurpose is a constant for the key in a label describing the purpose of the respective object.
	GardenerPurpose = "gardener.cloud/purpose"
	// GardenPurposeMachineClass is a constant for the 'machineclass' value in a label.
	GardenPurposeMachineClass = "machineclass"

	// GardenerOperation is a constant for an annotation on a resource that describes a desired operation.
	GardenerOperation = "gardener.cloud/operation"
	// GardenerOperationReconcile is a constant for the value of the operation annotation describing a reconcile
	// operation.
	GardenerOperationReconcile = "reconcile"

	// GardenRole is a constant for a label that describes a role.
	GardenRole = "gardener.cloud/role"
	// GardenRoleExtension is a constant for a label that describes the 'extensions' role.
	GardenRoleExtension = "extension"

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
	// LabelAPIServer is a constant for a label for the kube-controller-manager.
	LabelControllerManager = "controller-manager"
	// LabelAPIServer is a constant for a label for the kube-scheduler.
	LabelScheduler = "scheduler"

	// OperatingSystemConfigUnitNameKubeletService is a constant for a unit in the operating system config that contains the kubelet service.
	OperatingSystemConfigUnitNameKubeletService = "kubelet.service"
	// OperatingSystemConfigFilePathKernelSettings is a constant for a path to a file in the operating system config that contains some general kernel settings.
	OperatingSystemConfigFilePathKernelSettings = "/etc/sysctl.d/99-k8s-general.conf"
	// OperatingSystemConfigFilePathKubeletConfig is a constant for a path to a file in the operating system config that contains the kubelet configuration.
	OperatingSystemConfigFilePathKubeletConfig = "/var/lib/kubelet/config/kubelet"

	// ControllerRegistrationName is the key of a label on extension namespaces that indicates the controller registration name.
	LabelControllerRegistrationName = "controllerregistration.core.gardener.cloud/name"
)
