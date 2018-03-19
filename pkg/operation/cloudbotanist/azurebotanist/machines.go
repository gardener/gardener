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

	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/terraformer"
)

// GetMachineClassInfo returns the name of the class kind, the plural of it and the name of the Helm chart which
// contains the machine class template.
func (b *AzureBotanist) GetMachineClassInfo() (classKind, classPlural, classChartName string) {
	classKind = "AzureMachineClass"
	classPlural = "azuremachineclasses"
	classChartName = "azure-machineclass"
	return
}

// GenerateMachineConfig generates the configuration values for the cloud-specific machine class Helm chart. It
// also generates a list of corresponding MachineDeployments. It returns the computed list of MachineClasses and
// MachineDeployments.
func (b *AzureBotanist) GenerateMachineConfig() ([]map[string]interface{}, []operation.MachineDeployment, error) {
	var (
		resourceGroupName = "resourceGroupName"
		vnetName          = "vnetName"
		subnetName        = "subnetName"
		availabilitySetID = "availabilitySetID"
		outputVariables   = []string{resourceGroupName, vnetName, subnetName, availabilitySetID}
		workers           = b.Shoot.Info.Spec.Cloud.Azure.Workers

		machineDeployments = []operation.MachineDeployment{}
		machineClasses     = []map[string]interface{}{}
	)

	stateVariables, err := terraformer.New(b.Operation, common.TerraformerPurposeInfra).GetStateOutputVariables(outputVariables...)
	if err != nil {
		return nil, nil, err
	}

	for _, worker := range workers {
		cloudConfig, err := b.ComputeDownloaderCloudConfig(worker.Name)
		if err != nil {
			return nil, nil, err
		}

		machineClassSpec := map[string]interface{}{
			"region":            b.Shoot.Info.Spec.Cloud.Region,
			"resourceGroup":     stateVariables[resourceGroupName],
			"vnetName":          stateVariables[vnetName],
			"subnetName":        stateVariables[subnetName],
			"availabilitySetID": stateVariables[availabilitySetID],
			"tags": map[string]interface{}{
				"Name": b.Shoot.SeedNamespace,
				fmt.Sprintf("kubernetes.io-cluster-%s", b.Shoot.SeedNamespace): "1",
				"kubernetes.io-role-node":                                      "1",
			},
			"secret": map[string]interface{}{
				"cloudConfig": cloudConfig.FileContent("cloud-config.yaml"),
			},
			"machineType": worker.MachineType,
			"image": map[string]interface{}{
				"publisher": b.Shoot.Info.Spec.Cloud.Azure.MachineImage.Publisher,
				"offer":     b.Shoot.Info.Spec.Cloud.Azure.MachineImage.Offer,
				"sku":       b.Shoot.Info.Spec.Cloud.Azure.MachineImage.SKU,
				"version":   b.Shoot.Info.Spec.Cloud.Azure.MachineImage.Version,
			},
			"volumeSize":   common.DiskSize(worker.VolumeSize),
			"sshPublicKey": string(b.Secrets["ssh-keypair"].Data["id_rsa.pub"]),
		}

		var (
			machineClassSpecHash = common.MachineClassHash(machineClassSpec, b.Shoot.KubernetesMajorMinorVersion)
			deploymentName       = fmt.Sprintf("%s-%s", b.Shoot.SeedNamespace, worker.Name)
			className            = fmt.Sprintf("%s-%s", deploymentName, machineClassSpecHash)
		)

		machineDeployments = append(machineDeployments, operation.MachineDeployment{
			Name:      deploymentName,
			ClassName: className,
			Replicas:  worker.AutoScalerMax,
		})

		machineClassSpec["name"] = className
		machineClassSpec["secret"].(map[string]interface{})["clientID"] = string(b.Shoot.Secret.Data[ClientID])
		machineClassSpec["secret"].(map[string]interface{})["clientSecret"] = string(b.Shoot.Secret.Data[ClientSecret])
		machineClassSpec["secret"].(map[string]interface{})["subscriptionID"] = string(b.Shoot.Secret.Data[SubscriptionID])
		machineClassSpec["secret"].(map[string]interface{})["tenantID"] = string(b.Shoot.Secret.Data[TenantID])

		machineClasses = append(machineClasses, machineClassSpec)
	}

	return machineClasses, machineDeployments, nil
}
