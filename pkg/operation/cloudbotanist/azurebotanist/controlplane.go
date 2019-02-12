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
	"github.com/gardener/gardener/pkg/utils"
)

const cloudProviderConfigTemplate = `
cloud: AZUREPUBLICCLOUD
tenantId: %q
subscriptionId: %q
resourceGroup: %q
location: %q
vnetName: %q
subnetName: %q
securityGroupName: %q
routeTableName: %q
primaryAvailabilitySetName: %q
aadClientId: %q
aadClientSecret: %q
cloudProviderBackoff: true
cloudProviderBackoffRetries: 6
cloudProviderBackoffExponent: 1.5
cloudProviderBackoffDuration: 5
cloudProviderBackoffJitter: 1.0
cloudProviderRateLimit: true
cloudProviderRateLimitQPS: 10.0
cloudProviderRateLimitBucket: 100
cloudProviderRateLimitQPSWrite: 10.0
cloudProviderRateLimitBucketWrite: 100
`

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
	tf, err := b.NewShootTerraformer(common.TerraformerPurposeInfra)
	if err != nil {
		return "", err
	}
	stateVariables, err := tf.GetStateOutputVariables(resourceGroupName, vnetName, subnetName, routeTableName, availabilitySetName, securityGroupName)
	if err != nil {
		return "", err
	}

	cloudProviderConfig := fmt.Sprintf(
		cloudProviderConfigTemplate,
		string(b.Shoot.Secret.Data[TenantID]),
		string(b.Shoot.Secret.Data[SubscriptionID]),
		stateVariables[resourceGroupName],
		b.Shoot.Info.Spec.Cloud.Region,
		stateVariables[vnetName],
		stateVariables[subnetName],
		stateVariables[securityGroupName],
		stateVariables[routeTableName],
		stateVariables[availabilitySetName],
		string(b.Shoot.Secret.Data[ClientID]),
		string(b.Shoot.Secret.Data[ClientSecret]),
	)

	return cloudProviderConfig, nil
}

// RefreshCloudProviderConfig refreshes the cloud provider credentials in the existing cloud
// provider config.
func (b *AzureBotanist) RefreshCloudProviderConfig(currentConfig map[string]string) map[string]string {
	var (
		existing  = currentConfig[common.CloudProviderConfigMapKey]
		updated   = existing
		separator = ": "
	)

	updated = common.ReplaceCloudProviderConfigKey(updated, separator, "tenantId", string(b.Shoot.Secret.Data[TenantID]))
	updated = common.ReplaceCloudProviderConfigKey(updated, separator, "subscriptionId", string(b.Shoot.Secret.Data[SubscriptionID]))
	updated = common.ReplaceCloudProviderConfigKey(updated, separator, "aadClientId", string(b.Shoot.Secret.Data[ClientID]))
	updated = common.ReplaceCloudProviderConfigKey(updated, separator, "aadClientSecret", string(b.Shoot.Secret.Data[ClientSecret]))

	return map[string]string{
		common.CloudProviderConfigMapKey: updated,
	}
}

// GenerateKubeAPIServerServiceConfig generates the cloud provider specific values which are required to render the
// Service manifest of the kube-apiserver-service properly.
func (b *AzureBotanist) GenerateKubeAPIServerServiceConfig() (map[string]interface{}, error) {
	var values map[string]interface{}

	seedK8s112, err := utils.CompareVersions(b.K8sSeedClient.Version(), ">=", "1.12")
	if err != nil {
		return nil, err
	}

	if seedK8s112 {
		values = map[string]interface{}{
			"annotations": map[string]interface{}{
				"service.beta.kubernetes.io/azure-load-balancer-tcp-idle-timeout": "30",
			},
		}
	}

	return values, nil
}

// GenerateKubeAPIServerExposeConfig defines the cloud provider specific values which configure how the kube-apiserver
// is exposed to the public.
func (b *AzureBotanist) GenerateKubeAPIServerExposeConfig() (map[string]interface{}, error) {
	return map[string]interface{}{
		"advertiseAddress": b.APIServerAddress,
		"additionalParameters": []string{
			fmt.Sprintf("--external-hostname=%s", b.APIServerAddress),
		},
	}, nil
}

// GenerateKubeAPIServerConfig generates the cloud provider specific values which are required to render the
// Deployment manifest of the kube-apiserver properly.
func (b *AzureBotanist) GenerateKubeAPIServerConfig() (map[string]interface{}, error) {
	return nil, nil
}

// GenerateCloudControllerManagerConfig generates the cloud provider specific values which are required to
// render the Deployment manifest of the cloud-controller-manager properly.
func (b *AzureBotanist) GenerateCloudControllerManagerConfig() (map[string]interface{}, string, error) {
	return nil, common.CloudControllerManagerDeploymentName, nil
}

// GenerateCSIConfig generates the configuration for CSI charts
func (b *AzureBotanist) GenerateCSIConfig() (map[string]interface{}, error) {
	return nil, nil
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
	tf, err := b.NewBackupInfrastructureTerraformer()
	if err != nil {
		return nil, nil, err
	}
	stateVariables, err := tf.GetStateOutputVariables(storageAccountName, storageAccessKey, containerName)
	if err != nil {
		return nil, nil, err
	}

	secretData := map[string][]byte{
		"storage-account": []byte(stateVariables[storageAccountName]),
		"storage-key":     []byte(stateVariables[storageAccessKey]),
	}

	backupConfigData := map[string]interface{}{
		"schedule":         b.Operation.ShootBackup.Schedule,
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

// DeployCloudSpecificControlPlane does currently nothing for Azure.
func (b *AzureBotanist) DeployCloudSpecificControlPlane() error {
	return nil
}
