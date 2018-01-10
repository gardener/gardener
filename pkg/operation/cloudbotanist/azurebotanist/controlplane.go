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

package azurebotanist

import (
	"fmt"

	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/terraformer"
	"github.com/gardener/gardener/pkg/utils"
)

// GenerateCloudProviderConfig returns a cloud provider config for the Azure cloud provider
// as defined here: https://github.com/kubernetes/kubernetes/blob/release-1.7/pkg/cloudprovider/providers/azure/azure.go#L58.
func (b *AzureBotanist) GenerateCloudProviderConfig() (string, error) {
	stateConfigMap, err := terraformer.New(b.Operation, common.TerraformerPurposeInfra).GetState()
	if err != nil {
		return "", err
	}
	state := utils.ConvertJSONToMap(stateConfigMap)

	resourceGroupName, err := state.String("modules", "0", "outputs", "resourceGroupName", "value")
	if err != nil {
		return "", err
	}
	vnetName, err := state.String("modules", "0", "outputs", "vnetName", "value")
	if err != nil {
		return "", err
	}

	return `cloud: AZUREPUBLICCLOUD
tenantId: ` + string(b.Shoot.Secret.Data[TenantID]) + `
subscriptionId: ` + string(b.Shoot.Secret.Data[SubscriptionID]) + `
resourceGroup: ` + resourceGroupName + `
location: ` + b.Shoot.Info.Spec.Cloud.Region + `
vnetName: ` + vnetName + `
subnetName: workers
securityGroupName: nodes
routeTableName: worker_route_table
primaryAvailabilitySetName: workers-avset
aadClientId: ` + string(b.Shoot.Secret.Data[ClientID]) + `
aadClientSecret: ` + string(b.Shoot.Secret.Data[ClientSecret]) + `
cloudProviderBackoff: true
cloudProviderBackoffRetries: 6
cloudProviderBackoffExponent: 1.5
cloudProviderBackoffDuration: 120
cloudProviderBackoffJitter: 1.0
cloudProviderRateLimit: true
cloudProviderRateLimitQPS: 0.5
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
		"AdditionalParameters": []string{
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

// DeployAutoNodeRepair deploys the auto-node-repair into the Seed cluster. It primary job is to repair
// unHealthy Nodes by replacing them by newer ones - Not needed on Azure yet. To be implemented later
func (b *AzureBotanist) DeployAutoNodeRepair() error {
	return nil
}

// GenerateEtcdBackupSecretData generates the data for the secret which is required by the etcd-operator to
// store the backups on the ABS container, i.e. the secret contains the Azure storage account and the respective access key.
func (b *AzureBotanist) GenerateEtcdBackupSecretData() (map[string][]byte, error) {
	stateConfigMap, err := terraformer.New(b.Operation, common.TerraformerPurposeBackup).GetState()
	if err != nil {
		return nil, err
	}
	state := utils.ConvertJSONToMap(stateConfigMap)

	storageAccountName, err := state.String("modules", "0", "outputs", "storageAccountName", "value")
	if err != nil {
		return nil, err
	}

	storageAccessKey, err := state.String("modules", "0", "outputs", "storageAccessKey", "value")
	if err != nil {
		return nil, err
	}

	secretData := map[string][]byte{
		"storage-account": []byte(storageAccountName),
		"storage-key":     []byte(storageAccessKey),
	}

	return secretData, nil
}

// GenerateEtcdConfig returns the etcd deployment configuration (including backup settings) for the etcd
// Helm chart.
func (b *AzureBotanist) GenerateEtcdConfig(secretName string) (map[string]interface{}, error) {
	stateConfigMap, err := terraformer.New(b.Operation, common.TerraformerPurposeBackup).GetState()
	if err != nil {
		return nil, err
	}
	state := utils.ConvertJSONToMap(stateConfigMap)

	containerName, err := state.String("modules", "0", "outputs", "containerName", "value")
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"kind": "EtcdCluster",
		"backup": map[string]interface{}{
			"backupIntervalInSecond": b.Shoot.Info.Spec.Backup.IntervalInSecond,
			"maxBackups":             b.Shoot.Info.Spec.Backup.Maximum,
			"storageType":            "ABS",
			"abs": map[string]interface{}{
				"absContainer": containerName,
				"absSecret":    secretName,
			},
		},
	}, nil
}
