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
	"github.com/gardener/gardener/pkg/operation/terraformer"
	"github.com/gardener/gardener/pkg/utils"
)

// GenerateCloudProviderConfig generates the OpenStack cloud provider config.
// See this for more details:
// https://github.com/kubernetes/kubernetes/blob/master/pkg/cloudprovider/providers/openstack/openstack.go
func (b *OpenStackBotanist) GenerateCloudProviderConfig() (string, error) {
	var (
		floatingNetworkID = "floating_network_id"
		subnetID          = "subnet_id"
	)

	stateVariables, err := terraformer.New(b.Operation, common.TerraformerPurposeInfra).GetStateOutputVariables(floatingNetworkID, subnetID)
	if err != nil {
		return "", err
	}

	cloudProviderConfig := `
[Global]
auth-url=` + b.Shoot.CloudProfile.Spec.OpenStack.KeyStoneURL + `
domain-name=` + string(b.Shoot.Secret.Data[DomainName]) + `
tenant-name=` + string(b.Shoot.Secret.Data[TenantName]) + `
username=` + string(b.Shoot.Secret.Data[UserName]) + `
password=` + string(b.Shoot.Secret.Data[Password]) + `
[LoadBalancer]
lb-version=v2
lb-provider=` + b.Shoot.Info.Spec.Cloud.OpenStack.LoadBalancerProvider + `
floating-network-id=` + stateVariables[floatingNetworkID] + `
subnet-id=` + stateVariables[subnetID] + `
create-monitor=true
monitor-delay=60s
monitor-timeout=30s
monitor-max-retries=5`

	k8s1101, err := utils.CompareVersions(b.Shoot.Info.Spec.Kubernetes.Version, "^", "1.10.1")
	if err != nil {
		return "", err
	}

	if k8s1101 && b.Shoot.CloudProfile.Spec.OpenStack.DHCPDomain != nil {
		cloudProviderConfig += `
[Metadata]
dhcp-domain=` + *b.Shoot.CloudProfile.Spec.OpenStack.DHCPDomain
	}

	return cloudProviderConfig, nil
}

// GenerateKubeAPIServerConfig generates the cloud provider specific values which are required to render the
// Deployment manifest of the kube-apiserver properly.
func (b *OpenStackBotanist) GenerateKubeAPIServerConfig() (map[string]interface{}, error) {
	loadBalancerIP, err := utils.WaitUntilDNSNameResolvable(b.APIServerAddress)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"additionalParameters": []string{
			fmt.Sprintf("--external-hostname=%s", loadBalancerIP),
		},
	}, nil
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
	stateVariables, err := terraformer.New(b.Operation, common.TerraformerPurposeBackup).GetStateOutputVariables(containerName)
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
		"schedule":         b.Shoot.Info.Spec.Backup.Schedule,
		"maxBackups":       b.Shoot.Info.Spec.Backup.Maximum,
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
