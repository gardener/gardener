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

package openstackbotanist

import (
	"fmt"

	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
)

const cloudProviderConfigTemplate = `
[Global]
auth-url=%q
domain-name=%q
tenant-name=%q
username=%q
password=%q
[LoadBalancer]
lb-version=v2
lb-provider=%q
floating-network-id=%q
subnet-id=%q
create-monitor=true
monitor-delay=60s
monitor-timeout=30s
monitor-max-retries=5
`

// GenerateCloudProviderConfig generates the OpenStack cloud provider config.
// See this for more details:
// https://github.com/kubernetes/kubernetes/blob/master/pkg/cloudprovider/providers/openstack/openstack.go
func (b *OpenStackBotanist) GenerateCloudProviderConfig() (string, error) {
	var (
		floatingNetworkID = "floating_network_id"
		subnetID          = "subnet_id"
	)
	tf, err := b.NewShootTerraformer(common.TerraformerPurposeInfra)
	if err != nil {
		return "", err
	}
	stateVariables, err := tf.GetStateOutputVariables(floatingNetworkID, subnetID)
	if err != nil {
		return "", err
	}

	cloudProviderConfig := fmt.Sprintf(
		cloudProviderConfigTemplate,
		b.Shoot.CloudProfile.Spec.OpenStack.KeyStoneURL,
		string(b.Shoot.Secret.Data[DomainName]),
		string(b.Shoot.Secret.Data[TenantName]),
		string(b.Shoot.Secret.Data[UserName]),
		string(b.Shoot.Secret.Data[Password]),
		b.Shoot.Info.Spec.Cloud.OpenStack.LoadBalancerProvider,
		stateVariables[floatingNetworkID],
		stateVariables[subnetID],
	)

	// https://github.com/kubernetes/kubernetes/pull/63903#issue-188306465
	needsDHCPDomain, err := utils.CheckVersionMeetsConstraint(b.Shoot.Info.Spec.Kubernetes.Version, ">= 1.10.1, < 1.10.3")
	if err != nil {
		return "", err
	}

	if needsDHCPDomain && b.Shoot.CloudProfile.Spec.OpenStack.DHCPDomain != nil || b.Shoot.CloudProfile.Spec.OpenStack.RequestTimeout != nil {
		cloudProviderConfig += fmt.Sprintf(`
[Metadata]`)
	}

	if needsDHCPDomain && b.Shoot.CloudProfile.Spec.OpenStack.DHCPDomain != nil {
		cloudProviderConfig += fmt.Sprintf(`
dhcp-domain=%q`, *b.Shoot.CloudProfile.Spec.OpenStack.DHCPDomain)
	}

	if b.Shoot.CloudProfile.Spec.OpenStack.RequestTimeout != nil {
		cloudProviderConfig += fmt.Sprintf(`
request-timeout=%s`, *b.Shoot.CloudProfile.Spec.OpenStack.RequestTimeout)
	}

	return cloudProviderConfig, nil
}

// RefreshCloudProviderConfig refreshes the cloud provider credentials in the existing cloud
// provider config.
func (b *OpenStackBotanist) RefreshCloudProviderConfig(currentConfig map[string]string) map[string]string {
	var (
		existing  = currentConfig[common.CloudProviderConfigMapKey]
		updated   = existing
		separator = "="
	)

	updated = common.ReplaceCloudProviderConfigKey(updated, separator, "auth-url", b.Shoot.CloudProfile.Spec.OpenStack.KeyStoneURL)
	updated = common.ReplaceCloudProviderConfigKey(updated, separator, "domain-name", string(b.Shoot.Secret.Data[DomainName]))
	updated = common.ReplaceCloudProviderConfigKey(updated, separator, "tenant-name", string(b.Shoot.Secret.Data[TenantName]))
	updated = common.ReplaceCloudProviderConfigKey(updated, separator, "username", string(b.Shoot.Secret.Data[UserName]))
	updated = common.ReplaceCloudProviderConfigKey(updated, separator, "password", string(b.Shoot.Secret.Data[Password]))

	return map[string]string{
		common.CloudProviderConfigMapKey: updated,
	}
}

// GenerateKubeAPIServerServiceConfig generates the cloud provider specific values which are required to render the
// Service manifest of the kube-apiserver-service properly.
func (b *OpenStackBotanist) GenerateKubeAPIServerServiceConfig() (map[string]interface{}, error) {
	return nil, nil
}

// GenerateKubeAPIServerExposeConfig defines the cloud provider specific values which configure how the kube-apiserver
// is exposed to the public.
func (b *OpenStackBotanist) GenerateKubeAPIServerExposeConfig() (map[string]interface{}, error) {
	return map[string]interface{}{
		"advertiseAddress": b.APIServerAddress,
		"additionalParameters": []string{
			fmt.Sprintf("--external-hostname=%s", b.APIServerAddress),
		},
	}, nil
}

// GenerateKubeAPIServerConfig generates the cloud provider specific values which are required to render the
// Deployment manifest of the kube-apiserver properly.
func (b *OpenStackBotanist) GenerateKubeAPIServerConfig() (map[string]interface{}, error) {
	return nil, nil
}

// GenerateCloudControllerManagerConfig generates the cloud provider specific values which are required to
// render the Deployment manifest of the cloud-controller-manager properly.
func (b *OpenStackBotanist) GenerateCloudControllerManagerConfig() (map[string]interface{}, string, error) {
	return nil, common.CloudControllerManagerDeploymentName, nil
}

// GenerateCSIConfig generates the configuration for CSI charts
func (b *OpenStackBotanist) GenerateCSIConfig() (map[string]interface{}, error) {
	return nil, nil
}

// GenerateKubeControllerManagerConfig generates the cloud provider specific values which are required to
// render the Deployment manifest of the kube-controller-manager properly.
func (b *OpenStackBotanist) GenerateKubeControllerManagerConfig() (map[string]interface{}, error) {
	return nil, nil
}

// GenerateKubeSchedulerConfig generates the cloud provider specific values which are required to render the
// Deployment manifest of the kube-scheduler properly.
func (b *OpenStackBotanist) GenerateKubeSchedulerConfig() (map[string]interface{}, error) {
	return nil, nil
}

// GenerateEtcdBackupConfig returns the etcd backup configuration for the etcd Helm chart.
func (b *OpenStackBotanist) GenerateEtcdBackupConfig() (map[string][]byte, map[string]interface{}, error) {
	containerName := "containerName"

	tf, err := b.NewBackupInfrastructureTerraformer()
	if err != nil {
		return nil, nil, err
	}

	stateVariables, err := tf.GetStateOutputVariables(containerName)
	if err != nil {
		return nil, nil, err
	}

	secretData := map[string][]byte{
		UserName:   b.Seed.Secret.Data[UserName],
		Password:   b.Seed.Secret.Data[Password],
		TenantName: b.Seed.Secret.Data[TenantName],
		AuthURL:    []byte(b.Seed.CloudProfile.Spec.OpenStack.KeyStoneURL),
		DomainName: b.Seed.Secret.Data[DomainName],
	}

	backupConfigData := map[string]interface{}{
		"schedule":         b.Operation.ShootBackup.Schedule,
		"storageProvider":  "Swift",
		"storageContainer": stateVariables[containerName],
		"env": []map[string]interface{}{
			{
				"name": "OS_AUTH_URL",
				"valueFrom": map[string]interface{}{
					"secretKeyRef": map[string]interface{}{
						"name": common.BackupSecretName,
						"key":  AuthURL,
					},
				},
			},
			{
				"name": "OS_DOMAIN_NAME",
				"valueFrom": map[string]interface{}{
					"secretKeyRef": map[string]interface{}{
						"name": common.BackupSecretName,
						"key":  DomainName,
					},
				},
			},
			{
				"name": "OS_USERNAME",
				"valueFrom": map[string]interface{}{
					"secretKeyRef": map[string]interface{}{
						"name": common.BackupSecretName,
						"key":  UserName,
					},
				},
			},
			{
				"name": "OS_PASSWORD",
				"valueFrom": map[string]interface{}{
					"secretKeyRef": map[string]interface{}{
						"name": common.BackupSecretName,
						"key":  Password,
					},
				},
			},
			{
				"name": "OS_TENANT_NAME",
				"valueFrom": map[string]interface{}{
					"secretKeyRef": map[string]interface{}{
						"name": common.BackupSecretName,
						"key":  TenantName,
					},
				},
			},
		},
		"volumeMount": []map[string]interface{}{},
	}
	return secretData, backupConfigData, nil
}

// DeployCloudSpecificControlPlane does currently nothing for OpenStack.
func (b *OpenStackBotanist) DeployCloudSpecificControlPlane() error {
	return nil
}
