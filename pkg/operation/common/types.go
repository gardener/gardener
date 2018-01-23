// Copyright 2018 The Gardener Authors.
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
	// authenticate against the respective cloud provider (required by etcd-operator to store the backups
	// of Shoot clusters).
	BackupSecretName = "etcd-backup"

	// ChartPath is the path to the Helm charts.
	ChartPath = "charts"

	// ConfirmationDeletionTimestamp is an annotation on a Shoot resource whose value must be set equal to the Shoot's
	// '.metadata.deletionTimestamp' value to trigger the deletion process of the Shoot cluster.
	ConfirmationDeletionTimestamp = "confirmation.garden.sapcloud.io/deletionTimestamp"

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

	// GardenNamespace is a constant for the Garden namespace which holds configuration for the Gardener.
	GardenNamespace = "garden"

	// GardenRole is the key for an annotation on a Kubernetes Secret object whose value must be either 'seed' or
	// 'shoot'.
	GardenRole = "garden.sapcloud.io/role"

	// GardenRoleSeed is the value of the GardenRole key indicating type 'seed'.
	GardenRoleSeed = "seed"

	// GardenRoleDefaultDomain is the value of the GardenRole key indicating type 'default-domain'.
	GardenRoleDefaultDomain = "default-domain"

	// GardenRoleInternalDomain is the value of the GardenRole key indicating type 'internal-domain'.
	GardenRoleInternalDomain = "internal-domain"

	// GardenRoleImagePull is the value of the GardenRole key indicating type 'image-pull'.
	GardenRoleImagePull = "image-pull"

	// GardenRoleAlertingSMTP is the value of the GardenRole key indicating type 'alerting-smtp'.
	GardenRoleAlertingSMTP = "alerting-smtp"

	// GardenRoleMembers ist the value of GardenRole key indicating type 'members'.
	GardenRoleMembers = "members"

	//GardenRoleProject is the value of GardenRole key indicating type 'project'.
	GardenRoleProject = "project"

	// GardenOperatedBy is the key for an annotation of a Shoot cluster whose value must be a valid email address and
	// is used to send alerts to.
	GardenOperatedBy = "garden.sapcloud.io/operatedBy"

	// KubeAPIServerDeploymentName is the name of the kube-apiserver deployment.
	KubeAPIServerDeploymentName = "kube-apiserver"

	// KubeAddonManagerDeploymentName is the name of the kube-addon-manager deployment.
	KubeAddonManagerDeploymentName = "kube-addon-manager"

	// ProjectPrefix is the prefix of namespaces in the Garden cluster which is used for all projects created by the
	// Gardener UI.
	ProjectPrefix = "garden-"

	// PrometheusDeploymentName is the name of the Prometheus deployment.
	PrometheusDeploymentName = "prometheus"

	// CloudPurposeShoot is a constant used while instantiating a cloud botanist for the Shoot cluster.
	CloudPurposeShoot = "shoot"

	// CloudPurposeSeed is a constant used while instantiating a cloud botanist for the Seed cluster.
	CloudPurposeSeed = "seed"

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

	// ShootUseAsSeed is a constant for an annotation on a Shoot resource indicating that the Shoot shall be registered as Seed in the
	// Garden cluster once successfully created.
	ShootUseAsSeed = "shoot.garden.sapcloud.io/use-as-seed"
)

// CloudConfigUserDataConfig is a struct containing cloud-specific configuration required to
// render the shoot-cloud-config chart properly.
type CloudConfigUserDataConfig struct {
	CloudConfig       bool
	KubeletParameters []string
	NetworkPlugin     string
	CABundle          string
	WorkerNames       []string
}
