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

package awsbotanist

import (
	"fmt"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/terraformer"
)

// DeployInfrastructure kicks off a Terraform job which deploys the infrastructure.
func (b *AWSBotanist) DeployInfrastructure() error {
	var (
		createIGW         = true
		createVPC         = true
		internetGatewayID = "${aws_internet_gateway.igw.id}"
		vpcID             = b.Shoot.Info.Spec.Cloud.AWS.Networks.VPC.ID
	)
	// check if we should use an existing VPC or create a new one
	if vpcID != "" {
		vpcExists, err := b.AWSClient.CheckIfVPCExists(vpcID)
		if err != nil {
			return err
		}
		createVPC = !vpcExists

		// check if we should use the existing IGW or create a new one
		if vpcExists {
			igwID, err := b.AWSClient.GetInternetGateway(vpcID)
			if err != nil {
				return err
			}
			if igwID != "" {
				internetGatewayID = igwID
				createIGW = true
			}
		}
	} else {
		vpcID = "${aws_vpc.vpc.id}"
	}

	machineImage, err := findAMIForRegion(b.Shoot.Info.Spec.Cloud.Region, b.Shoot.CloudProfile.Spec.AWS.MachineImages)
	if err != nil {
		return err
	}

	return terraformer.
		New(b.Operation, common.TerraformerPurposeInfra).
		SetVariablesEnvironment(b.generateTerraformInfraVariablesEnvironment()).
		DefineConfig("aws-infra", b.generateTerraformInfraConfig(createVPC, createIGW, vpcID, internetGatewayID, machineImage)).
		Apply()
}

// DestroyInfrastructure kicks off a Terraform job which destroys the infrastructure.
func (b *AWSBotanist) DestroyInfrastructure() error {
	return terraformer.
		New(b.Operation, common.TerraformerPurposeInfra).
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
func (b *AWSBotanist) generateTerraformInfraConfig(createVPC, createIGW bool, vpcID, internetGatewayID string, machineImage gardenv1beta1.AWSMachineImage) map[string]interface{} {
	var (
		sshSecret                   = b.Secrets["ssh-keypair"]
		cloudConfigDownloaderSecret = b.Secrets["cloud-config-downloader"]
		dhcpDomainName              = "ec2.internal"
		workers                     = distributeWorkersOverZones(b.Shoot.Info.Spec.Cloud.AWS.Workers, b.Shoot.Info.Spec.Cloud.AWS.Zones)
		zones                       = []map[string]interface{}{}
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
			"igw": createIGW,
			"clusterAutoscalerPolicies": b.Shoot.Info.Spec.Addons.ClusterAutoscaler.Enabled && !b.Shoot.Info.Spec.Addons.Kube2IAM.Enabled,
		},
		"sshPublicKey": string(sshSecret.Data["id_rsa.pub"]),
		"vpc": map[string]interface{}{
			"id":                vpcID,
			"cidr":              b.Shoot.Info.Spec.Cloud.AWS.Networks.VPC.CIDR,
			"dhcpDomainName":    dhcpDomainName,
			"internetGatewayID": internetGatewayID,
		},
		"clusterName": b.Shoot.SeedNamespace,
		"coreOSImage": machineImage.AMI,
		"cloudConfig": map[string]interface{}{
			"kubeconfig": string(cloudConfigDownloaderSecret.Data["kubeconfig"]),
		},
		"workers": workers,
		"zones":   zones,
	}
}

// DeployBackupInfrastructure kicks off a Terraform job which deploys the infrastructure resources for backup.
// It sets up the User and the Bucket to store the backups. Allocate permission to the User to access the bucket.
func (b *AWSBotanist) DeployBackupInfrastructure() error {
	return terraformer.
		New(b.Operation, common.TerraformerPurposeBackup).
		SetVariablesEnvironment(b.generateTerraformBackupVariablesEnvironment()).
		DefineConfig("aws-backup", b.generateTerraformBackupConfig()).
		Apply()
}

// DestroyBackupInfrastructure kicks off a Terraform job which destroys the infrastructure for etcd backup.
func (b *AWSBotanist) DestroyBackupInfrastructure() error {
	return terraformer.
		New(b.Operation, common.TerraformerPurposeBackup).
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
			"name": b.Shoot.SeedNamespace,
		},
		"clusterName": b.Shoot.SeedNamespace,
	}
}

func findAMIForRegion(region string, machineImages []gardenv1beta1.AWSMachineImage) (gardenv1beta1.AWSMachineImage, error) {
	for _, machineImage := range machineImages {
		if machineImage.Region == region {
			return machineImage, nil
		}
	}
	return gardenv1beta1.AWSMachineImage{}, fmt.Errorf("could not find an AMI for region %s", region)
}

// distributeWorkersOverZones distributes the worker groups over the zones equally and returns a map
// which can be injected into a Helm chart.
func distributeWorkersOverZones(workerList []gardenv1beta1.AWSWorker, zoneList []string) []map[string]interface{} {
	var (
		workers = []map[string]interface{}{}
		zoneLen = len(zoneList)
	)

	for _, worker := range workerList {
		var workerZones = []map[string]interface{}{}
		for zoneIndex, zone := range zoneList {
			workerZones = append(workerZones, map[string]interface{}{
				"name":          zone,
				"autoScalerMin": common.DistributeOverZones(zoneIndex, worker.AutoScalerMin, zoneLen),
				"autoScalerMax": common.DistributeOverZones(zoneIndex, worker.AutoScalerMax, zoneLen),
			})
		}

		workers = append(workers, map[string]interface{}{
			"name":        worker.Name,
			"machineType": worker.MachineType,
			"volumeType":  worker.VolumeType,
			"volumeSize":  common.DiskSize(worker.VolumeSize),
			"zones":       workerZones,
		})
	}

	return workers
}
