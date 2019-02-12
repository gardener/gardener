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

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
)

// DeployInfrastructure kicks off a Terraform job which deploys the infrastructure.
func (b *AzureBotanist) DeployInfrastructure() error {
	var (
		createResourceGroup = true
		createVNet          = true
		resourceGroupName   = b.Shoot.SeedNamespace
		vnetName            = b.Shoot.SeedNamespace
		vnetCIDR            = b.Shoot.Info.Spec.Cloud.Azure.Networks.Workers
	)

	// check if we should use an existing ResourceGroup or create a new one
	if b.Shoot.Info.Spec.Cloud.Azure.ResourceGroup != nil {
		createResourceGroup = false
		resourceGroupName = b.Shoot.Info.Spec.Cloud.Azure.ResourceGroup.Name
	}

	// check if we should use an existing ResourceGroup or create a new one
	if b.Shoot.Info.Spec.Cloud.Azure.Networks.VNet.Name != nil {
		createVNet = false
		vnetName = *b.Shoot.Info.Spec.Cloud.Azure.Networks.VNet.Name
	}
	if b.Shoot.Info.Spec.Cloud.Azure.Networks.VNet.CIDR != nil {
		vnetCIDR = *b.Shoot.Info.Spec.Cloud.Azure.Networks.VNet.CIDR
	}

	countUpdateDomains, err := findDomainCountForRegion(b.Shoot.Info.Spec.Cloud.Region, b.Shoot.CloudProfile.Spec.Azure.CountUpdateDomains)
	if err != nil {
		return err
	}
	countFaultDomains, err := findDomainCountForRegion(b.Shoot.Info.Spec.Cloud.Region, b.Shoot.CloudProfile.Spec.Azure.CountFaultDomains)
	if err != nil {
		return err
	}
	tf, err := b.NewShootTerraformer(common.TerraformerPurposeInfra)
	if err != nil {
		return err
	}
	return tf.
		SetVariablesEnvironment(b.generateTerraformInfraVariablesEnvironment()).
		InitializeWith(b.ChartInitializer("azure-infra", b.generateTerraformInfraConfig(createResourceGroup, createVNet, resourceGroupName, vnetName, vnetCIDR, countUpdateDomains, countFaultDomains))).
		Apply()
}

// DestroyInfrastructure kicks off a Terraform job which destroys the infrastructure.
func (b *AzureBotanist) DestroyInfrastructure() error {
	tf, err := b.NewShootTerraformer(common.TerraformerPurposeInfra)
	if err != nil {
		return err
	}
	return tf.
		SetVariablesEnvironment(b.generateTerraformInfraVariablesEnvironment()).
		Destroy()
}

// generateTerraformInfraVariablesEnvironment generates the environment containing the credentials which
// are required to validate/apply/destroy the Terraform configuration. These environment must contain
// Terraform variables which are prefixed with TF_VAR_.
func (b *AzureBotanist) generateTerraformInfraVariablesEnvironment() map[string]string {
	return common.GenerateTerraformVariablesEnvironment(b.Shoot.Secret, map[string]string{
		"CLIENT_ID":     ClientID,
		"CLIENT_SECRET": ClientSecret,
	})
}

// generateTerraformInfraConfig creates the Terraform variables and the Terraform config (for the infrastructure)
// and returns them (these values will be stored as a ConfigMap and a Secret in the Garden cluster.
func (b *AzureBotanist) generateTerraformInfraConfig(createResourceGroup, createVNet bool, resourceGroupName, vnetName string, vnetCIDR gardenv1beta1.CIDR, countUpdateDomains, countFaultDomains gardenv1beta1.AzureDomainCount) map[string]interface{} {
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
		"resourceGroup": map[string]interface{}{
			"name": resourceGroupName,
			"vnet": map[string]interface{}{
				"name": vnetName,
				"cidr": vnetCIDR,
			},
		},
		"clusterName": b.Shoot.SeedNamespace,
		"networks": map[string]interface{}{
			"worker": b.Shoot.Info.Spec.Cloud.Azure.Networks.Workers,
		},
	}
}

// DeployBackupInfrastructure kicks off a Terraform job which creates the infrastructure resources for backup.
func (b *AzureBotanist) DeployBackupInfrastructure() error {
	tf, err := b.NewBackupInfrastructureTerraformer()
	if err != nil {
		return err
	}
	return tf.
		SetVariablesEnvironment(b.generateTerraformBackupVariablesEnvironment()).
		InitializeWith(b.ChartInitializer("azure-backup", b.generateTerraformBackupConfig())).
		Apply()
}

// DestroyBackupInfrastructure kicks off a Terraform job which destroys the infrastructure for backup.
func (b *AzureBotanist) DestroyBackupInfrastructure() error {
	tf, err := b.NewBackupInfrastructureTerraformer()
	if err != nil {
		return err
	}
	return tf.
		SetVariablesEnvironment(b.generateTerraformBackupVariablesEnvironment()).
		Destroy()
}

// generateTerraformBackupVariablesEnvironment generates the environment containing the credentials which
// are required to validate/apply/destroy the Terraform configuration. These environment must contain
// Terraform variables which are prefixed with TF_VAR_.
func (b *AzureBotanist) generateTerraformBackupVariablesEnvironment() map[string]string {
	return common.GenerateTerraformVariablesEnvironment(b.Seed.Secret, map[string]string{
		"CLIENT_ID":     ClientID,
		"CLIENT_SECRET": ClientSecret,
	})
}

// generateTerraformBackupConfig creates the Terraform variables and the Terraform config (for the backup)
// and returns them.
func (b *AzureBotanist) generateTerraformBackupConfig() map[string]interface{} {
	var (
		shootUIDSHA       = utils.ComputeSHA1Hex([]byte(b.BackupInfrastructure.Spec.ShootUID))
		resourceGroupName string
	)

	// TODO: Remove this and use only "--" as separator, once we have all shoots deployed as per new naming conventions.
	if common.IsFollowingNewNamingConvention(b.BackupInfrastructure.Name) {
		resourceGroupName = fmt.Sprintf("backup--%s", b.BackupInfrastructure.Name)
	} else {
		resourceGroupName = fmt.Sprintf("%s-backup-%s", common.ExtractShootName(b.BackupInfrastructure.Name), shootUIDSHA[:15])
	}

	return map[string]interface{}{
		"azure": map[string]interface{}{
			"subscriptionID":     string(b.Seed.Secret.Data[SubscriptionID]),
			"tenantID":           string(b.Seed.Secret.Data[TenantID]),
			"region":             b.Seed.Info.Spec.Cloud.Region,
			"storageAccountName": fmt.Sprintf("bkp%s", shootUIDSHA[:15]),
			"resourceGroupName":  resourceGroupName,
		},
		"clusterName": b.BackupInfrastructure.Name,
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
