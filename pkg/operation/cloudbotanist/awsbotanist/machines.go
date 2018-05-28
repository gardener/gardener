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

	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/terraformer"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
)

// GetMachineClassInfo returns the name of the class kind, the plural of it and the name of the Helm chart which
// contains the machine class template.
func (b *AWSBotanist) GetMachineClassInfo() (classKind, classPlural, classChartName string) {
	classKind = "AWSMachineClass"
	classPlural = "awsmachineclasses"
	classChartName = "aws-machineclass"
	return
}

// GenerateMachineClassSecretData generates the secret data for the machine class secret (except the userData field
// which is computed elsewhere).
func (b *AWSBotanist) GenerateMachineClassSecretData() map[string][]byte {
	return map[string][]byte{
		machinev1alpha1.AWSAccessKeyID:     b.Shoot.Secret.Data[AccessKeyID],
		machinev1alpha1.AWSSecretAccessKey: b.Shoot.Secret.Data[SecretAccessKey],
	}
}

// GenerateMachineConfig generates the configuration values for the cloud-specific machine class Helm chart. It
// also generates a list of corresponding MachineDeployments. The provided worker groups will be distributed over
// the desired availability zones. It returns the computed list of MachineClasses and MachineDeployments.
func (b *AWSBotanist) GenerateMachineConfig() ([]map[string]interface{}, []operation.MachineDeployment, error) {
	var (
		iamInstanceProfile = "iamInstanceProfileNodes"
		keyName            = "keyName"
		securityGroup      = "security_group_nodes"
		outputVariables    = []string{iamInstanceProfile, keyName, securityGroup}
		workers            = b.Shoot.Info.Spec.Cloud.AWS.Workers
		zones              = b.Shoot.Info.Spec.Cloud.AWS.Zones
		zoneLen            = len(zones)

		machineDeployments = []operation.MachineDeployment{}
		machineClasses     = []map[string]interface{}{}

		tfOutputNameSubnet = func(zoneIndex int) string {
			return fmt.Sprintf("subnet_nodes_z%d", zoneIndex)
		}
	)

	for zoneIndex := range zones {
		outputVariables = append(outputVariables, tfOutputNameSubnet(zoneIndex))
	}

	stateVariables, err := terraformer.NewFromOperation(b.Operation, common.TerraformerPurposeInfra).GetStateOutputVariables(outputVariables...)
	if err != nil {
		return nil, nil, err
	}

	for zoneIndex := range zones {
		for _, worker := range workers {
			cloudConfig, err := b.ComputeDownloaderCloudConfig(worker.Name)
			if err != nil {
				return nil, nil, err
			}

			machineClassSpec := map[string]interface{}{
				"ami":                b.Shoot.Info.Spec.Cloud.AWS.MachineImage.AMI,
				"region":             b.Shoot.Info.Spec.Cloud.Region,
				"machineType":        worker.MachineType,
				"iamInstanceProfile": stateVariables[iamInstanceProfile],
				"keyName":            stateVariables[keyName],
				"networkInterfaces": []map[string]interface{}{
					{
						"subnetID":         stateVariables[tfOutputNameSubnet(zoneIndex)],
						"securityGroupIDs": []string{stateVariables[securityGroup]},
					},
				},
				"tags": map[string]string{
					fmt.Sprintf("kubernetes.io/cluster/%s", b.Shoot.SeedNamespace): "1",
					"kubernetes.io/role/node":                                      "1",
				},
				"secret": map[string]interface{}{
					"cloudConfig": cloudConfig.FileContent("cloud-config.yaml"),
				},
				"blockDevices": []map[string]interface{}{
					{
						"ebs": map[string]interface{}{
							"volumeSize": common.DiskSize(worker.VolumeSize),
							"volumeType": worker.VolumeType,
						},
					},
				},
			}

			var (
				machineClassSpecHash = common.MachineClassHash(machineClassSpec, b.Shoot.KubernetesMajorMinorVersion)
				deploymentName       = fmt.Sprintf("%s-%s-z%d", b.Shoot.SeedNamespace, worker.Name, zoneIndex+1)
				className            = fmt.Sprintf("%s-%s", deploymentName, machineClassSpecHash)
				secretData           = b.GenerateMachineClassSecretData()
			)

			machineDeployments = append(machineDeployments, operation.MachineDeployment{
				Name:      deploymentName,
				ClassName: className,
				Replicas:  common.DistributeOverZones(zoneIndex, worker.AutoScalerMax, zoneLen),
			})

			machineClassSpec["name"] = className
			machineClassSpec["secret"].(map[string]interface{})["accessKeyID"] = string(secretData[machinev1alpha1.AWSAccessKeyID])
			machineClassSpec["secret"].(map[string]interface{})["secretAccessKey"] = string(secretData[machinev1alpha1.AWSSecretAccessKey])

			machineClasses = append(machineClasses, machineClassSpec)
		}
	}

	return machineClasses, machineDeployments, nil
}
