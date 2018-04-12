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
	"fmt"

	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/terraformer"
)

// GetMachineClassInfo returns the name of the class kind, the plural of it and the name of the Helm chart which
// contains the machine class template.
func (b *OpenStackBotanist) GetMachineClassInfo() (classKind, classPlural, classChartName string) {
	classKind = "OpenStackMachineClass"
	classPlural = "openstackmachineclasses"
	classChartName = "openstack-machineclass"
	return
}

// GenerateMachineConfig generates the configuration values for the cloud-specific machine class Helm chart. It
// also generates a list of corresponding MachineDeployments. The provided worker groups will be distributed over
// the desired availability zones. It returns the computed list of MachineClasses and MachineDeployments.
func (b *OpenStackBotanist) GenerateMachineConfig() ([]map[string]interface{}, []operation.MachineDeployment, error) {
	var (
		networkID         = "network_id"
		keyName           = "key_name"
		securityGroupName = "security_group_name"
		outputVariables   = []string{networkID, keyName, securityGroupName}
		workers           = b.Shoot.Info.Spec.Cloud.OpenStack.Workers
		zones             = b.Shoot.Info.Spec.Cloud.OpenStack.Zones
		zoneLen           = len(zones)

		machineDeployments = []operation.MachineDeployment{}
		machineClasses     = []map[string]interface{}{}
	)

	stateVariables, err := terraformer.New(b.Operation, common.TerraformerPurposeInfra).GetStateOutputVariables(outputVariables...)
	if err != nil {
		return nil, nil, err
	}

	for zoneIndex, zone := range zones {
		for _, worker := range workers {
			cloudConfig, err := b.ComputeDownloaderCloudConfig(worker.Name)
			if err != nil {
				return nil, nil, err
			}

			machineClassSpec := map[string]interface{}{
				"region":           b.Shoot.Info.Spec.Cloud.Region,
				"availabilityZone": zone,
				"machineType":      worker.MachineType,
				"keyName":          stateVariables[keyName],
				"imageName":        b.Shoot.Info.Spec.Cloud.OpenStack.MachineImage.Image,
				"networkID":        stateVariables[networkID],
				"securityGroups":   []string{stateVariables[securityGroupName]},
				"tags": map[string]string{
					fmt.Sprintf("kubernetes.io-cluster-%s", b.Shoot.SeedNamespace): "1",
					"kubernetes.io-role-node":                                      "1",
				},
				"secret": map[string]interface{}{
					"cloudConfig": cloudConfig.FileContent("cloud-config.yaml"),
				},
			}

			var (
				machineClassSpecHash = common.MachineClassHash(machineClassSpec, b.Shoot.KubernetesMajorMinorVersion)
				deploymentName       = fmt.Sprintf("%s-%s-z%d", b.Shoot.SeedNamespace, worker.Name, zoneIndex+1)
				className            = fmt.Sprintf("%s-%s", deploymentName, machineClassSpecHash)
			)

			machineDeployments = append(machineDeployments, operation.MachineDeployment{
				Name:      deploymentName,
				ClassName: className,
				Replicas:  common.DistributeOverZones(zoneIndex, worker.AutoScalerMax, zoneLen),
			})

			machineClassSpec["name"] = className
			machineClassSpec["secret"].(map[string]interface{})["authURL"] = b.Shoot.CloudProfile.Spec.OpenStack.KeyStoneURL
			machineClassSpec["secret"].(map[string]interface{})["domainName"] = string(b.Shoot.Secret.Data[DomainName])
			machineClassSpec["secret"].(map[string]interface{})["tenantName"] = string(b.Shoot.Secret.Data[TenantName])
			machineClassSpec["secret"].(map[string]interface{})["username"] = string(b.Shoot.Secret.Data[UserName])
			machineClassSpec["secret"].(map[string]interface{})["password"] = string(b.Shoot.Secret.Data[Password])

			machineClasses = append(machineClasses, machineClassSpec)
		}
	}

	return machineClasses, machineDeployments, nil
}
