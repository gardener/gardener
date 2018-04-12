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

package azurebotanist

import (
	"fmt"

	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/terraformer"
	"github.com/gardener/gardener/pkg/utils"
)

// GenerateCloudProviderConfig returns a cloud provider config for the Azure cloud provider
// See this for more details:
// https://github.com/kubernetes/kubernetes/blob/master/pkg/cloudprovider/providers/azure/azure.go
func (b *AzureBotanist) GenerateCloudProviderConfig() (string, error) {
	var (
		resourceGroupName   = "resourceGroupName"
		vnetName            = "vnetName"
		availabilitySetName = "availabilitySetName"
		subnetName          = "subnetName"
		routeTableName      = "routeTableName"
		securityGroupName   = "securityGroupName"
	)
	stateVariables, err := terraformer.New(b.Operation, common.TerraformerPurposeInfra).GetStateOutputVariables(resourceGroupName, vnetName, subnetName, routeTableName, availabilitySetName, securityGroupName)
	if err != nil {
		return "", err
	}

	return `cloud: AZUREPUBLICCLOUD
tenantId: ` + string(b.Shoot.Secret.Data[TenantID]) + `
subscriptionId: ` + string(b.Shoot.Secret.Data[SubscriptionID]) + `
resourceGroup: ` + stateVariables[resourceGroupName] + `
location: ` + b.Shoot.Info.Spec.Cloud.Region + `
vnetName: ` + stateVariables[vnetName] + `
subnetName: ` + stateVariables[subnetName] + `
securityGroupName: ` + stateVariables[securityGroupName] + `
routeTableName: ` + stateVariables[routeTableName] + `
primaryAvailabilitySetName: ` + stateVariables[availabilitySetName] + `
aadClientId: ` + string(b.Shoot.Secret.Data[ClientID]) + `
aadClientSecret: ` + string(b.Shoot.Secret.Data[ClientSecret]) + `
cloudProviderBackoff: true
cloudProviderBackoffRetries: 6
cloudProviderBackoffExponent: 1.5
cloudProviderBackoffDuration: 5
cloudProviderBackoffJitter: 1.0
cloudProviderRateLimit: true
cloudProviderRateLimitQPS: 1.0
cloudProviderRateLimitBucket: 5`, nil
}

// GenerateKubeAPIServerConfig generates the cloud provider specific values which are required to render the
// Deployment manifest of the kube-apiserver properly.
func (b *AzureBotanist) GenerateKubeAPIServerConfig() (map[string]interface{}, error) {
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
func (b *AzureBotanist) GenerateKubeControllerManagerConfig() (map[string]interface{}, error) {
	return nil, nil
}

// GenerateKubeSchedulerConfig generates the cloud provider specific values which are required to render the
// Deployment manifest of the kube-scheduler properly.
func (b *AzureBotanist) GenerateKubeSchedulerConfig() (map[string]interface{}, error) {
	return nil, nil
}

// GenerateEtcdBackupConfig returns the etcd backup configuration for the etcd Helm chart.
func (b *AzureBotanist) GenerateEtcdBackupConfig() (map[string][]byte, map[string]interface{}, error) {
	var (
		storageAccountName = "storageAccountName"
		storageAccessKey   = "storageAccessKey"
		containerName      = "containerName"
	)
	stateVariables, err := terraformer.New(b.Operation, common.TerraformerPurposeBackup).GetStateOutputVariables(storageAccountName, storageAccessKey, containerName)
	if err != nil {
		return nil, nil, err
	}

	secretData := map[string][]byte{
		"storage-account": []byte(stateVariables[storageAccountName]),
		"storage-key":     []byte(stateVariables[storageAccessKey]),
	}

	backupConfigData := map[string]interface{}{
		"schedule":         b.Shoot.Info.Spec.Backup.Schedule,
		"maxBackups":       b.Shoot.Info.Spec.Backup.Maximum,
		"storageProvider":  "ABS",
		"backupSecret":     common.BackupSecretName,
		"storageContainer": stateVariables[containerName],
		"env": []map[string]interface{}{
			{
				"name": "STORAGE_ACCOUNT",
				"valueFrom": map[string]interface{}{
					"secretKeyRef": map[string]interface{}{
						"name": common.BackupSecretName,
						"key":  "storage-account",
					},
				},
			},
			{
				"name": "STORAGE_KEY",
				"valueFrom": map[string]interface{}{
					"secretKeyRef": map[string]interface{}{
						"name": common.BackupSecretName,
						"key":  "storage-key",
					},
				},
			},
		},
		"volumeMount": []map[string]interface{}{},
	}

	return secretData, backupConfigData, nil
}
