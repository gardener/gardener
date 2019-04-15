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
	return terraformer.GenerateVariablesEnvironment(b.Seed.Secret, map[string]string{
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
