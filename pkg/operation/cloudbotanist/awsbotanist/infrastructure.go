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

package awsbotanist

import (
	"fmt"

	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/terraformer"
)

// DeployInfrastructure kicks off a Terraform job which deploys the infrastructure.
func (b *AWSBotanist) DeployInfrastructure() error {
	var (
		createVPC         = true
		vpcID             = "${aws_vpc.vpc.id}"
		internetGatewayID = "${aws_internet_gateway.igw.id}"
		vpcCIDR           = ""
	)

	// check if we should use an existing VPC or create a new one
	if b.Shoot.Info.Spec.Cloud.AWS.Networks.VPC.ID != nil {
		createVPC = false
		vpcID = *b.Shoot.Info.Spec.Cloud.AWS.Networks.VPC.ID
		igwID, err := b.AWSClient.GetInternetGateway(vpcID)
		if err != nil {
			return err
		}
		internetGatewayID = igwID
	} else if b.Shoot.Info.Spec.Cloud.AWS.Networks.VPC.CIDR != nil {
		vpcCIDR = string(*b.Shoot.Info.Spec.Cloud.AWS.Networks.VPC.CIDR)
	}

	return terraformer.
		NewFromOperation(b.Operation, common.TerraformerPurposeInfra).
		SetVariablesEnvironment(b.generateTerraformInfraVariablesEnvironment()).
		DefineConfig("aws-infra", b.generateTerraformInfraConfig(createVPC, vpcID, internetGatewayID, vpcCIDR)).
		Apply()
}

// DestroyInfrastructure kicks off a Terraform job which destroys the infrastructure.
func (b *AWSBotanist) DestroyInfrastructure() error {
	return terraformer.
		NewFromOperation(b.Operation, common.TerraformerPurposeInfra).
		SetVariablesEnvironment(b.generateTerraformInfraVariablesEnvironment()).
		Destroy()
}

// generateTerraformInfraVariablesEnvironment generates the environment containing the credentials which
// are required to validate/apply/destroy the Terraform configuration. These environment must contain
// Terraform variables which are prefixed with TF_VAR_.
func (b *AWSBotanist) generateTerraformInfraVariablesEnvironment() []map[string]interface{} {
	return common.GenerateTerraformVariablesEnvironment(b.Shoot.Secret, map[string]string{
		"ACCESS_KEY_ID":     AccessKeyID,
		"SECRET_ACCESS_KEY": SecretAccessKey,
	})
}

// generateTerraformInfraConfig creates the Terraform variables and the Terraform config (for the infrastructure)
// and returns them (these values will be stored as a ConfigMap and a Secret in the Garden cluster.
func (b *AWSBotanist) generateTerraformInfraConfig(createVPC bool, vpcID, internetGatewayID, vpcCIDR string) map[string]interface{} {
	var (
		sshSecret      = b.Secrets["ssh-keypair"]
		dhcpDomainName = "ec2.internal"
		zones          = []map[string]interface{}{}
	)

	if b.Shoot.Info.Spec.Cloud.Region != "us-east-1" {
		dhcpDomainName = fmt.Sprintf("%s.compute.internal", b.Shoot.Info.Spec.Cloud.Region)
	}

	for zoneIndex, zone := range b.Shoot.Info.Spec.Cloud.AWS.Zones {
		zones = append(zones, map[string]interface{}{
			"name": zone,
			"cidr": map[string]interface{}{
				"worker":   b.Shoot.Info.Spec.Cloud.AWS.Networks.Workers[zoneIndex],
				"public":   b.Shoot.Info.Spec.Cloud.AWS.Networks.Public[zoneIndex],
				"internal": b.Shoot.Info.Spec.Cloud.AWS.Networks.Internal[zoneIndex],
			},
		})
	}

	return map[string]interface{}{
		"aws": map[string]interface{}{
			"region": b.Shoot.Info.Spec.Cloud.Region,
		},
		"create": map[string]interface{}{
			"vpc": createVPC,
			"clusterAutoscalerPolicies": b.Shoot.ClusterAutoscalerEnabled() && !b.Shoot.Kube2IAMEnabled(),
		},
		"sshPublicKey": string(sshSecret.Data["id_rsa.pub"]),
		"vpc": map[string]interface{}{
			"id":                vpcID,
			"cidr":              vpcCIDR,
			"dhcpDomainName":    dhcpDomainName,
			"internetGatewayID": internetGatewayID,
		},
		"clusterName": b.Shoot.SeedNamespace,
		"zones":       zones,
	}
}

// DeployBackupInfrastructure kicks off a Terraform job which deploys the infrastructure resources for backup.
// It sets up the User and the Bucket to store the backups. Allocate permission to the User to access the bucket.
func (b *AWSBotanist) DeployBackupInfrastructure() error {
	return terraformer.
		New(b.Logger, b.K8sSeedClient, common.TerraformerPurposeBackup, b.BackupInfrastructure.Name, common.GenerateBackupNamespaceName(b.BackupInfrastructure.Name), b.ImageVector).
		SetVariablesEnvironment(b.generateTerraformBackupVariablesEnvironment()).
		DefineConfig("aws-backup", b.generateTerraformBackupConfig()).
		Apply()
}

// DestroyBackupInfrastructure kicks off a Terraform job which destroys the infrastructure for etcd backup.
func (b *AWSBotanist) DestroyBackupInfrastructure() error {
	return terraformer.
		New(b.Logger, b.K8sSeedClient, common.TerraformerPurposeBackup, b.BackupInfrastructure.Name, common.GenerateBackupNamespaceName(b.BackupInfrastructure.Name), b.ImageVector).
		SetVariablesEnvironment(b.generateTerraformBackupVariablesEnvironment()).
		Destroy()
}

// generateTerraformBackupVariablesEnvironment generates the environment containing the credentials which
// are required to validate/apply/destroy the Terraform configuration. These environment must contain
// Terraform variables which are prefixed with TF_VAR_.
func (b *AWSBotanist) generateTerraformBackupVariablesEnvironment() []map[string]interface{} {
	return common.GenerateTerraformVariablesEnvironment(b.Seed.Secret, map[string]string{
		"ACCESS_KEY_ID":     AccessKeyID,
		"SECRET_ACCESS_KEY": SecretAccessKey,
	})
}

// generateTerraformBackupConfig creates the Terraform variables and the Terraform config (for the backup)
// and returns them.
func (b *AWSBotanist) generateTerraformBackupConfig() map[string]interface{} {
	return map[string]interface{}{
		"aws": map[string]interface{}{
			"region": b.Seed.Info.Spec.Cloud.Region,
		},
		"bucket": map[string]interface{}{
			"name": b.Operation.BackupInfrastructure.Name,
		},
		"clusterName": b.Operation.BackupInfrastructure.Name,
	}
}
