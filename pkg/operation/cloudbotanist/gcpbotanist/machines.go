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

package gcpbotanist

import (
	"fmt"

	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/terraformer"
)

// GetMachineClassInfo returns the name of the class kind, the plural of it and the name of the Helm chart which
// contains the machine class template.
func (b *GCPBotanist) GetMachineClassInfo() (classKind, classPlural, classChartName string) {
	classKind = "GCPMachineClass"
	classPlural = "gcpmachineclasses"
	classChartName = "gcp-machineclass"
	return
}

// GenerateMachineConfig generates the configuration values for the cloud-specific machine class Helm chart. It
// also generates a list of corresponding MachineDeployments. The provided worker groups will be distributed over
// the desired availability zones. It returns the computed list of MachineClasses and MachineDeployments.
func (b *GCPBotanist) GenerateMachineConfig() ([]map[string]interface{}, []operation.MachineDeployment, error) {
	var (
		serviceAccountEmail = "service_account_email"
		subnetNodes         = "subnet_nodes"
		outputVariables     = []string{serviceAccountEmail, subnetNodes}
		workers             = b.Shoot.Info.Spec.Cloud.GCP.Workers
		zones               = b.Shoot.Info.Spec.Cloud.GCP.Zones
		zoneLen             = len(zones)

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
				"region":             b.Shoot.Info.Spec.Cloud.Region,
				"zone":               zone,
				"canIpForward":       true,
				"deletionProtection": false,
				"description":        fmt.Sprintf("Machine of Shoot %s created by machine-controller-manager.", b.Shoot.Info.Name),
				"disks": []map[string]interface{}{
					{
						"autoDelete": true,
						"boot":       true,
						"sizeGb":     common.DiskSize(worker.VolumeSize),
						"type":       worker.VolumeType,
						"image":      b.Shoot.Info.Spec.Cloud.GCP.MachineImage.Image,
						"labels": map[string]interface{}{
							"name": b.Shoot.Info.Name,
						},
					},
				},
				"labels": map[string]interface{}{
					"name": b.Shoot.Info.Name,
				},
				"machineType": worker.MachineType,
				"networkInterfaces": []map[string]interface{}{
					{
						"subnetwork": stateVariables[subnetNodes],
					},
				},
				"scheduling": map[string]interface{}{
					"automaticRestart":  true,
					"onHostMaintenance": "MIGRATE",
					"preemptible":       false,
				},
				"secret": map[string]interface{}{
					"cloudConfig": cloudConfig.FileContent("cloud-config.yaml"),
				},
				"serviceAccounts": []map[string]interface{}{
					{
						"email": stateVariables[serviceAccountEmail],
						"scopes": []string{
							"https://www.googleapis.com/auth/compute",
						},
					},
				},
				"tags": []string{
					b.Shoot.SeedNamespace,
					fmt.Sprintf("kubernetes-io-cluster-%s", b.Shoot.SeedNamespace),
					"kubernetes-io-role-node",
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
			machineClassSpec["secret"].(map[string]interface{})["serviceAccountJSON"] = string(b.Shoot.Secret.Data[ServiceAccountJSON])

			machineClasses = append(machineClasses, machineClassSpec)
		}
	}

	return machineClasses, machineDeployments, nil
}
