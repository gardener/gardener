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

package openstackbotanist

import (
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/terraformer"
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

	return terraformer.
		New(b.Operation, common.TerraformerPurposeInfra).
		SetVariablesEnvironment(b.generateTerraformInfraVariablesEnvironment()).
		DefineConfig("openstack-infra", b.generateTerraformInfraConfig(createRouter, routerID)).
		Apply()
}

// DestroyInfrastructure kicks off a Terraform job which destroys the infrastructure.
func (b *OpenStackBotanist) DestroyInfrastructure() error {
	return terraformer.
		New(b.Operation, common.TerraformerPurposeInfra).
		SetVariablesEnvironment(b.generateTerraformInfraVariablesEnvironment()).
		Destroy()
}

// generateTerraformInfraVariablesEnvironment generates the environment containing the credentials which
// are required to validate/apply/destroy the Terraform configuration. These environment must contain
// Terraform variables which are prefixed with TF_VAR_.
func (b *OpenStackBotanist) generateTerraformInfraVariablesEnvironment() []map[string]interface{} {
	return common.GenerateTerraformVariablesEnvironment(b.Shoot.Secret, map[string]string{
		"USER_NAME": UserName,
		"PASSWORD":  Password,
	})
}

// generateTerraformInfraConfig creates the Terraform variables and the Terraform config (for the infrastructure)
// and returns them (these values will be stored as a ConfigMap and a Secret in the Garden cluster.
func (b *OpenStackBotanist) generateTerraformInfraConfig(createRouter bool, routerID string) map[string]interface{} {
	var (
		sshSecret                   = b.Secrets["ssh-keypair"]
		cloudConfigDownloaderSecret = b.Secrets["cloud-config-downloader"]
		workers                     = distributeWorkersOverZones(b.Shoot.Info.Spec.Cloud.OpenStack.Workers, b.Shoot.Info.Spec.Cloud.OpenStack.Zones)
		zones                       = []map[string]interface{}{}
	)

	for _, zone := range b.Shoot.Info.Spec.Cloud.OpenStack.Zones {
		zones = append(zones, map[string]interface{}{
			"name": zone,
		})
	}

	networks := map[string]interface{}{
		"worker": b.Shoot.Info.Spec.Cloud.OpenStack.Networks.Workers[0],
	}

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
		"sshPublicKey": string(sshSecret.Data["id_rsa.pub"]),
		"router": map[string]interface{}{
			"id": routerID,
		},
		"clusterName": b.Shoot.SeedNamespace,
		"coreOSImage": b.Shoot.CloudProfile.Spec.OpenStack.MachineImage.Name,
		"cloudConfig": map[string]interface{}{
			"kubeconfig": string(cloudConfigDownloaderSecret.Data["kubeconfig"]),
		},
		"networks": networks,
		"workers":  workers,
		"zones":    zones,
	}
}

// DeployBackupInfrastructure kicks off a Terraform job which creates the infrastructure resources for backup.
func (b *OpenStackBotanist) DeployBackupInfrastructure() error {
	return nil
}

// DestroyBackupInfrastructure kicks off a Terraform job which destroys the infrastructure for backup.
func (b *OpenStackBotanist) DestroyBackupInfrastructure() error {
	return nil
}

// distributeWorkersOverZones distributes the worker groups over the zones equally and returns a map
// which can be injected into a Helm chart.
func distributeWorkersOverZones(workerList []gardenv1beta1.OpenStackWorker, zoneList []string) []map[string]interface{} {
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
			"zones":       workerZones,
		})
	}

	return workers
}
