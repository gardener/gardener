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

package gcpbotanist

import (
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/terraformer"
)

// DeployInfrastructure kicks off a Terraform job which deploys the infrastructure.
func (b *GCPBotanist) DeployInfrastructure() error {
	var (
		vpcName   = "${google_compute_network.network.name}"
		createVPC = true
	)
	// check if we should use an existing VPC or create a new one
	if b.VPCName != "" {
		vpcName = b.VPCName
		createVPC = false
	}

	return terraformer.
		New(b.Operation, common.TerraformerPurposeInfra).
		SetVariablesEnvironment(b.generateTerraformInfraVariablesEnvironment()).
		DefineConfig("gcp-infra", b.generateTerraformInfraConfig(createVPC, vpcName)).
		Apply()
}

// DestroyInfrastructure kicks off a Terraform job which destroys the infrastructure.
func (b *GCPBotanist) DestroyInfrastructure() error {
	return terraformer.
		New(b.Operation, common.TerraformerPurposeInfra).
		SetVariablesEnvironment(b.generateTerraformInfraVariablesEnvironment()).
		Destroy()
}

// generateTerraformInfraVariablesEnvironment generates the environment containing the credentials which
// are required to validate/apply/destroy the Terraform configuration. These environment must contain
// Terraform variables which are prefixed with TF_VAR_.
func (b *GCPBotanist) generateTerraformInfraVariablesEnvironment() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":  "TF_VAR_SERVICEACCOUNT",
			"value": b.MinifiedServiceAccount,
		},
	}
}

// generateTerraformInfraConfig creates the Terraform variables and the Terraform config (for the infrastructure)
// and returns them (these values will be stored as a ConfigMap and a Secret in the Garden cluster.
func (b *GCPBotanist) generateTerraformInfraConfig(createVPC bool, vpcName string) map[string]interface{} {
	var (
		cloudConfigDownloaderSecret = b.Secrets["cloud-config-downloader"]
		workers                     = distributeWorkersOverZones(b.Shoot.Info.Spec.Cloud.GCP.Workers, b.Shoot.Info.Spec.Cloud.GCP.Zones)
	)

	networks := map[string]interface{}{
		"pods":     b.Shoot.GetPodNetwork(),
		"services": b.Shoot.GetServiceNetwork(),
		"worker":   b.Shoot.Info.Spec.Cloud.GCP.Networks.Workers[0],
	}

	return map[string]interface{}{
		"google": map[string]interface{}{
			"region":  b.Shoot.Info.Spec.Cloud.Region,
			"project": b.Project,
		},
		"create": map[string]interface{}{
			"vpc": createVPC,
		},
		"vpc": map[string]interface{}{
			"name": vpcName,
		},
		"clusterName": b.Shoot.SeedNamespace,
		"coreOSImage": b.Shoot.CloudProfile.Spec.GCP.MachineImage.Name,
		"cloudConfig": map[string]interface{}{
			"kubeconfig": string(cloudConfigDownloaderSecret.Data["kubeconfig"]),
		},
		"networks": networks,
		"workers":  workers,
	}
}

// DeployBackupInfrastructure kicks off a Terraform job which deploys the infrastructure resources for backup.
// TODO: implement backup functionality for GCP
func (b *GCPBotanist) DeployBackupInfrastructure() error {
	return nil
}

// DestroyBackupInfrastructure kicks off a Terraform job which destroys the infrastructure for backup.
// TODO: implement backup functionality for GCP
func (b *GCPBotanist) DestroyBackupInfrastructure() error {
	return nil
}

// distributeWorkersOverZones distributes the worker groups over the zones equally and returns a map
// which can be injected into a Helm chart.
func distributeWorkersOverZones(workerList []gardenv1beta1.GCPWorker, zoneList []string) []map[string]interface{} {
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
