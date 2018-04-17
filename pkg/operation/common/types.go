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

const (
	// AlertManagerDeploymentName is the name of the AlertManager deployment.
	AlertManagerDeploymentName = "alertmanager"

	// BackupSecretName defines the name of the secret containing the credentials which are required to
	// authenticate against the respective cloud provider (required to store the backups of Shoot clusters).
	BackupSecretName = "etcd-backup"

	// ChartPath is the path to the Helm charts.
	ChartPath = "charts"

	// CloudConfigPrefix is a constant for the prefix which is added to secret storing the original cloud config (which
	// is being downloaded from the cloud-config-downloader process)
	CloudConfigPrefix = "cloud-config"

	// CloudPurposeShoot is a constant used while instantiating a cloud botanist for the Shoot cluster.
	CloudPurposeShoot = "shoot"

	// CloudPurposeSeed is a constant used while instantiating a cloud botanist for the Seed cluster.
	CloudPurposeSeed = "seed"

	// ConfirmationDeletionTimestamp is an annotation on a Shoot resource whose value must be set equal to the Shoot's
	// '.metadata.deletionTimestamp' value to trigger the deletion process of the Shoot cluster.
	ConfirmationDeletionTimestamp = "confirmation.garden.sapcloud.io/deletionTimestamp"

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

	// EtcdRoleEvents is the constant defining the role for etcd storing events in Shoot.
	EtcdRoleEvents = "events"

	// GardenNamespace is the namespace in which the configuration and secrets for
	// the Gardener controller manager will be stored (e.g., secrets for the Seed clusters).
	// It is also used by the gardener-apiserver.
	GardenNamespace = "garden"

	// GardenRole is the key for an annotation on a Kubernetes object indicating what it is used for.
	GardenRole = "garden.sapcloud.io/role"

	// GardenRoleSeed is the value of the GardenRole key indicating type 'seed'.
	GardenRoleSeed = "seed"

	// GardenRoleDefaultDomain is the value of the GardenRole key indicating type 'default-domain'.
	GardenRoleDefaultDomain = "default-domain"

	// GardenRoleInternalDomain is the value of the GardenRole key indicating type 'internal-domain'.
	GardenRoleInternalDomain = "internal-domain"

	// GardenRoleAlertingSMTP is the value of the GardenRole key indicating type 'alerting-smtp'.
	GardenRoleAlertingSMTP = "alerting-smtp"

	// GardenRoleMembers ist the value of GardenRole key indicating type 'members'.
	GardenRoleMembers = "members"

	//GardenRoleProject is the value of GardenRole key indicating type 'project'.
	GardenRoleProject = "project"

	// GardenCreatedBy is the key for an annotation of a Shoot cluster whose value indicates contains the username
	// of the user that created the resource.
	GardenCreatedBy = "garden.sapcloud.io/createdBy"

	// GardenOperatedBy is the key for an annotation of a Shoot cluster whose value must be a valid email address and
	// is used to send alerts to.
	GardenOperatedBy = "garden.sapcloud.io/operatedBy"

	// GardenPurpose is a key for a label describing the purpose of the respective object.
	GardenPurpose = "garden.sapcloud.io/purpose"

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

	// KubeAddonManagerDeploymentName is the name of the kube-addon-manager deployment.
	KubeAddonManagerDeploymentName = "kube-addon-manager"

	// ProjectName is they key of a label on namespaces whose value holds the project name. Usually, the label is set
	// by the Gardener Dashboard.
	ProjectName = "project.garden.sapcloud.io/name"

	// PrometheusDeploymentName is the name of the Prometheus deployment.
	PrometheusDeploymentName = "prometheus"

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

	// ShootUnhealthy is a constant for a label on a Shoot resource indicating that the Shoot is unhealthy. It is set and unset by the
	// Shoot Care controller and can be used to easily identify Shoot clusters with issues.
	ShootUnhealthy = "shoot.garden.sapcloud.io/unhealthy"

	// ShootOperation is a constant for an annotation on a Shoot in a failed state indicating that the operation should be retried.
	ShootOperation = "shoot.garden.sapcloud.io/operation"

	// ShootSyncPeriod is a constant for an annotation on a Shoot which may be used to overwrite the global Shoot controller sync period.
	// The value must be a duration. It can also be used to disable the reconciliation at all by setting it to 0m. Disabling the reconciliation
	// does only mean that the period reconciliation is disabled. However, when the Gardener is restarted/redeployed or the specification is
	// changed then the reconciliation flow will be executed.
	ShootSyncPeriod = "shoot.garden.sapcloud.io/sync-period"

	// ShootIgnore is a constant for an annotation on a Shoot which may be used to tell the Gardener that the Shoot with this name should be
	// ignored completely. That means that the Shoot will never reach the reconciliation flow (independent of the operation (create/update/
	// delete)).
	ShootIgnore = "shoot.garden.sapcloud.io/ignore"
)

// CloudConfigUserDataConfig is a struct containing cloud-specific configuration required to
// render the shoot-cloud-config chart properly.
type CloudConfigUserDataConfig struct {
	ProvisionCloudProviderConfig bool
	KubeletParameters            []string
	WorkerNames                  []string
	HostnameOverride             bool
}
