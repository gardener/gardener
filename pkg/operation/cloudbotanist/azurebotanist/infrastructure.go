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

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/terraformer"
	"github.com/gardener/gardener/pkg/utils"
)

// DeployInfrastructure kicks off a Terraform job which deploys the infrastructure.
func (b *AzureBotanist) DeployInfrastructure() error {
	var (
		createResourceGroup = true
		createVNet          = true
		resourceGroupName   = fmt.Sprintf("%s-sapk8s", b.Shoot.SeedNamespace)
		vnetName            = fmt.Sprintf("%s-vnet", b.Shoot.SeedNamespace)
		vnetCIDR            = b.Shoot.Info.Spec.Cloud.Azure.Networks.Workers
	)
	// check if we should use an existing ResourceGroup or create a new one
	if b.Shoot.Info.Spec.Cloud.Azure.ResourceGroup != nil {
		createResourceGroup = false
		resourceGroupName = b.Shoot.Info.Spec.Cloud.Azure.ResourceGroup.Name
	}
	// check if we should use an existing ResourceGroup or create a new one
	if b.Shoot.Info.Spec.Cloud.Azure.Networks.VNet.Name != "" {
		createVNet = false
		vnetName = b.Shoot.Info.Spec.Cloud.Azure.Networks.VNet.Name
	}
	if b.Shoot.Info.Spec.Cloud.Azure.Networks.VNet.CIDR != "" {
		vnetCIDR = b.Shoot.Info.Spec.Cloud.Azure.Networks.VNet.CIDR
	}

	countUpdateDomains, err := findDomainCountForRegion(b.Shoot.Info.Spec.Cloud.Region, b.Shoot.CloudProfile.Spec.Azure.CountUpdateDomains)
	if err != nil {
		return err
	}
	countFaultDomains, err := findDomainCountForRegion(b.Shoot.Info.Spec.Cloud.Region, b.Shoot.CloudProfile.Spec.Azure.CountFaultDomains)
	if err != nil {
		return err
	}

	return terraformer.
		New(b.Operation, common.TerraformerPurposeInfra).
		SetVariablesEnvironment(b.generateTerraformInfraVariablesEnvironment()).
		DefineConfig("azure-infra", b.generateTerraformInfraConfig(createResourceGroup, createVNet, resourceGroupName, vnetName, vnetCIDR, countUpdateDomains, countFaultDomains)).
		Apply()
}

// DestroyInfrastructure kicks off a Terraform job which destroys the infrastructure.
func (b *AzureBotanist) DestroyInfrastructure() error {
	return terraformer.
		New(b.Operation, common.TerraformerPurposeInfra).
		SetVariablesEnvironment(b.generateTerraformInfraVariablesEnvironment()).
		Destroy()
}

// generateTerraformInfraVariablesEnvironment generates the environment containing the credentials which
// are required to validate/apply/destroy the Terraform configuration. These environment must contain
// Terraform variables which are prefixed with TF_VAR_.
func (b *AzureBotanist) generateTerraformInfraVariablesEnvironment() []map[string]interface{} {
	return common.GenerateTerraformVariablesEnvironment(b.Shoot.Secret, map[string]string{
		"CLIENT_ID":     ClientID,
		"CLIENT_SECRET": ClientSecret,
	})
}

// generateTerraformInfraConfig creates the Terraform variables and the Terraform config (for the infrastructure)
// and returns them (these values will be stored as a ConfigMap and a Secret in the Garden cluster.
func (b *AzureBotanist) generateTerraformInfraConfig(createResourceGroup, createVNet bool, resourceGroupName, vnetName string, vnetCIDR gardenv1beta1.CIDR, countUpdateDomains, countFaultDomains gardenv1beta1.AzureDomainCount) map[string]interface{} {
	var (
		sshSecret                   = b.Secrets["ssh-keypair"]
		cloudConfigDownloaderSecret = b.Secrets["cloud-config-downloader"]
		workers                     = []map[string]interface{}{}
	)

	networks := map[string]interface{}{
		"worker": b.Shoot.Info.Spec.Cloud.Azure.Networks.Workers,
	}
	if b.Shoot.Info.Spec.Cloud.Azure.Networks.Public != nil {
		networks["public"] = *(b.Shoot.Info.Spec.Cloud.Azure.Networks.Public)
	}

	for _, worker := range b.Shoot.Info.Spec.Cloud.Azure.Workers {
		workers = append(workers, map[string]interface{}{
			"name":          worker.Name,
			"machineType":   worker.MachineType,
			"volumeType":    worker.VolumeType,
			"volumeSize":    common.DiskSize(worker.VolumeSize),
			"autoScalerMin": worker.AutoScalerMin,
			"autoScalerMax": worker.AutoScalerMax,
		})
	}

	return map[string]interface{}{
		"azure": map[string]interface{}{
			"subscriptionID":     string(b.Shoot.Secret.Data[SubscriptionID]),
			"tenantID":           string(b.Shoot.Secret.Data[TenantID]),
			"region":             b.Shoot.Info.Spec.Cloud.Region,
			"countUpdateDomains": countUpdateDomains.Count,
			"countFaultDomains":  countFaultDomains.Count,
		},
		"create": map[string]interface{}{
			"resourceGroup": createResourceGroup,
			"vnet":          createVNet,
		},
		"sshPublicKey": string(sshSecret.Data["id_rsa.pub"]),
		"resourceGroup": map[string]interface{}{
			"name": resourceGroupName,
			"vnet": map[string]interface{}{
				"name": vnetName,
				"cidr": vnetCIDR,
			},
		},
		"clusterName": b.Shoot.SeedNamespace,
		"coreOSImage": map[string]interface{}{
			"sku":     b.Shoot.CloudProfile.Spec.Azure.MachineImage.Channel,
			"version": b.Shoot.CloudProfile.Spec.Azure.MachineImage.Version,
		},
		"cloudConfig": map[string]interface{}{
			"kubeconfig": string(cloudConfigDownloaderSecret.Data["kubeconfig"]),
		},
		"networks": networks,
		"workers":  workers,
	}
}

// DeployBackupInfrastructure kicks off a Terraform job which creates the infrastructure resources for backup.
func (b *AzureBotanist) DeployBackupInfrastructure() error {
	return terraformer.
		New(b.Operation, common.TerraformerPurposeBackup).
		SetVariablesEnvironment(b.generateTerraformBackupVariablesEnvironment()).
		DefineConfig("azure-backup", b.generateTerraformBackupConfig()).
		Apply()
}

// DestroyBackupInfrastructure kicks off a Terraform job which destroys the infrastructure for backup.
func (b *AzureBotanist) DestroyBackupInfrastructure() error {
	return terraformer.
		New(b.Operation, common.TerraformerPurposeBackup).
		SetVariablesEnvironment(b.generateTerraformBackupVariablesEnvironment()).
		Destroy()
}

// generateTerraformBackupVariablesEnvironment generates the environment containing the credentials which
// are required to validate/apply/destroy the Terraform configuration. These environment must contain
// Terraform variables which are prefixed with TF_VAR_.
func (b *AzureBotanist) generateTerraformBackupVariablesEnvironment() []map[string]interface{} {
	return common.GenerateTerraformVariablesEnvironment(b.Seed.Secret, map[string]string{
		"CLIENT_ID":     ClientID,
		"CLIENT_SECRET": ClientSecret,
	})
}

// generateTerraformBackupConfig creates the Terraform variables and the Terraform config (for the backup)
// and returns them.
func (b *AzureBotanist) generateTerraformBackupConfig() map[string]interface{} {
	return map[string]interface{}{
		"azure": map[string]interface{}{
			"subscriptionID":     string(b.Seed.Secret.Data[SubscriptionID]),
			"tenantID":           string(b.Seed.Secret.Data[TenantID]),
			"region":             b.Seed.Info.Spec.Cloud.Region,
			"storageAccountName": fmt.Sprintf("bkp%s", utils.ComputeSHA1Hex([]byte(b.Shoot.Info.Status.UID))[:15]),
		},
		"clusterName": b.Shoot.SeedNamespace,
	}
}

func findDomainCountForRegion(region string, domainCounts []gardenv1beta1.AzureDomainCount) (gardenv1beta1.AzureDomainCount, error) {
	for _, domainCount := range domainCounts {
		if domainCount.Region == region {
			return domainCount, nil
		}
	}
	return gardenv1beta1.AzureDomainCount{}, fmt.Errorf("could not find a domain count for region %s", region)
}
