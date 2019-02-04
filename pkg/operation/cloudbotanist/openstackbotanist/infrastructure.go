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
	"github.com/gardener/gardener/pkg/operation/common"
)

// DeployInfrastructure kicks off a Terraform job which deploys the infrastructure.
func (b *OpenStackBotanist) DeployInfrastructure() error {
	var (
		routerID     = "${openstack_networking_router_v2.router.id}"
		createRouter = true
	)

	// check if we should use an existing Router or create a new one
	if b.Shoot.Info.Spec.Cloud.OpenStack.Networks.Router != nil {
		routerID = b.Shoot.Info.Spec.Cloud.OpenStack.Networks.Router.ID
		createRouter = false
	}
	tf, err := b.NewShootTerraformer(common.TerraformerPurposeInfra)
	if err != nil {
		return err
	}
	return tf.
		SetVariablesEnvironment(b.generateTerraformInfraVariablesEnvironment()).
		InitializeWith(b.ChartInitializer("openstack-infra", b.generateTerraformInfraConfig(createRouter, routerID))).
		Apply()
}

// DestroyInfrastructure kicks off a Terraform job which destroys the infrastructure.
func (b *OpenStackBotanist) DestroyInfrastructure() error {
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
func (b *OpenStackBotanist) generateTerraformInfraVariablesEnvironment() map[string]string {
	return common.GenerateTerraformVariablesEnvironment(b.Shoot.Secret, map[string]string{
		"USER_NAME": UserName,
		"PASSWORD":  Password,
	})
}

// generateTerraformInfraConfig creates the Terraform variables and the Terraform config (for the infrastructure)
// and returns them (these values will be stored as a ConfigMap and a Secret in the Garden cluster.
func (b *OpenStackBotanist) generateTerraformInfraConfig(createRouter bool, routerID string) map[string]interface{} {
	return map[string]interface{}{
		"openstack": map[string]interface{}{
			"authURL":              b.Shoot.CloudProfile.Spec.OpenStack.KeyStoneURL,
			"domainName":           string(b.Shoot.Secret.Data[DomainName]),
			"tenantName":           string(b.Shoot.Secret.Data[TenantName]),
			"region":               b.Shoot.Info.Spec.Cloud.Region,
			"floatingPoolName":     b.Shoot.Info.Spec.Cloud.OpenStack.FloatingPoolName,
			"loadBalancerProvider": b.Shoot.Info.Spec.Cloud.OpenStack.LoadBalancerProvider,
		},
		"create": map[string]interface{}{
			"router": createRouter,
		},
		"dnsServers":   b.Shoot.CloudProfile.Spec.OpenStack.DNSServers,
		"sshPublicKey": string(b.Secrets["ssh-keypair"].Data["id_rsa.pub"]),
		"router": map[string]interface{}{
			"id": routerID,
		},
		"clusterName": b.Shoot.SeedNamespace,
		"networks": map[string]interface{}{
			"worker": b.Shoot.Info.Spec.Cloud.OpenStack.Networks.Workers[0],
		},
	}
}

// DeployBackupInfrastructure kicks off a Terraform job which creates the infrastructure resources for backup.
func (b *OpenStackBotanist) DeployBackupInfrastructure() error {
	tf, err := b.NewBackupInfrastructureTerraformer()
	if err != nil {
		return err
	}
	return tf.
		SetVariablesEnvironment(b.generateTerraformBackupVariablesEnvironment()).
		InitializeWith(b.ChartInitializer("openstack-backup", b.generateTerraformBackupConfig())).
		Apply()
}

// DestroyBackupInfrastructure kicks off a Terraform job which destroys the infrastructure for backup.
func (b *OpenStackBotanist) DestroyBackupInfrastructure() error {
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
func (b *OpenStackBotanist) generateTerraformBackupVariablesEnvironment() map[string]string {
	return common.GenerateTerraformVariablesEnvironment(b.Seed.Secret, map[string]string{
		"USER_NAME": UserName,
		"PASSWORD":  Password,
	})
}

// generateTerraformBackupConfig creates the Terraform variables and the Terraform config (for the backup)
// and returns them.
func (b *OpenStackBotanist) generateTerraformBackupConfig() map[string]interface{} {
	return map[string]interface{}{
		"openstack": map[string]interface{}{
			"authURL":    b.Seed.CloudProfile.Spec.OpenStack.KeyStoneURL,
			"domainName": string(b.Seed.Secret.Data[DomainName]),
			"tenantName": string(b.Seed.Secret.Data[TenantName]),
			"region":     b.Seed.Info.Spec.Cloud.Region,
		},
		"container": map[string]interface{}{
			"name": b.Operation.BackupInfrastructure.Name,
		},
		"clusterName": b.Operation.BackupInfrastructure.Name,
	}
}
