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
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// GetMachineClassInfo returns the name of the class kind, the plural of it and the name of the Helm chart which
// contains the machine class template.
func (b *OpenStackBotanist) GetMachineClassInfo() (classKind, classPlural, classChartName string) {
	classKind = "OpenStackMachineClass"
	classPlural = "openstackmachineclasses"
	classChartName = "openstack-machineclass"
	return
}

// GenerateMachineClassSecretData generates the secret data for the machine class secret (except the userData field
// which is computed elsewhere).
func (b *OpenStackBotanist) GenerateMachineClassSecretData() map[string][]byte {
	return map[string][]byte{
		machinev1alpha1.OpenStackAuthURL:    []byte(b.Shoot.CloudProfile.Spec.OpenStack.KeyStoneURL),
		machinev1alpha1.OpenStackInsecure:   []byte("true"),
		machinev1alpha1.OpenStackDomainName: b.Shoot.Secret.Data[DomainName],
		machinev1alpha1.OpenStackTenantName: b.Shoot.Secret.Data[TenantName],
		machinev1alpha1.OpenStackUsername:   b.Shoot.Secret.Data[UserName],
		machinev1alpha1.OpenStackPassword:   b.Shoot.Secret.Data[Password],
	}
}

// GenerateMachineConfig generates the configuration values for the cloud-specific machine class Helm chart. It
// also generates a list of corresponding MachineDeployments. The provided worker groups will be distributed over
// the desired availability zones. It returns the computed list of MachineClasses and MachineDeployments.
func (b *OpenStackBotanist) GenerateMachineConfig() ([]map[string]interface{}, operation.MachineDeployments, error) {
	var (
		networkID         = "network_id"
		keyName           = "key_name"
		securityGroupName = "security_group_name"
		outputVariables   = []string{networkID, keyName, securityGroupName}
		workers           = b.Shoot.Info.Spec.Cloud.OpenStack.Workers
		zones             = b.Shoot.Info.Spec.Cloud.OpenStack.Zones
		zoneLen           = len(zones)

		machineDeployments = operation.MachineDeployments{}
		machineClasses     = []map[string]interface{}{}
	)
	tf, err := b.NewShootTerraformer(common.TerraformerPurposeInfra)
	if err != nil {
		return nil, nil, err
	}
	stateVariables, err := tf.GetStateOutputVariables(outputVariables...)
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
				"podNetworkCidr":   b.Shoot.GetPodNetwork(),
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
				secretData           = b.GenerateMachineClassSecretData()
			)

			machineDeployments = append(machineDeployments, operation.MachineDeployment{
				Name:           deploymentName,
				ClassName:      className,
				Minimum:        common.DistributeOverZones(zoneIndex, worker.AutoScalerMin, zoneLen),
				Maximum:        common.DistributeOverZones(zoneIndex, worker.AutoScalerMax, zoneLen),
				MaxSurge:       common.DistributePositiveIntOrPercent(zoneIndex, *worker.MaxSurge, zoneLen, worker.AutoScalerMax),
				MaxUnavailable: common.DistributePositiveIntOrPercent(zoneIndex, *worker.MaxUnavailable, zoneLen, worker.AutoScalerMin),
			})

			machineClassSpec["name"] = className
			machineClassSpec["secret"].(map[string]interface{})["authURL"] = string(secretData[machinev1alpha1.OpenStackAuthURL])
			machineClassSpec["secret"].(map[string]interface{})["domainName"] = string(secretData[machinev1alpha1.OpenStackDomainName])
			machineClassSpec["secret"].(map[string]interface{})["tenantName"] = string(secretData[machinev1alpha1.OpenStackTenantName])
			machineClassSpec["secret"].(map[string]interface{})["username"] = string(secretData[machinev1alpha1.OpenStackUsername])
			machineClassSpec["secret"].(map[string]interface{})["password"] = string(secretData[machinev1alpha1.OpenStackPassword])

			machineClasses = append(machineClasses, machineClassSpec)
		}
	}

	return machineClasses, machineDeployments, nil
}

// ListMachineClasses returns two sets of strings whereas the first contains the names of all machine
// classes, and the second the names of all referenced secrets.
func (b *OpenStackBotanist) ListMachineClasses() (sets.String, sets.String, error) {
	var (
		classNames  = sets.NewString()
		secretNames = sets.NewString()
	)

	existingMachineClasses, err := b.K8sSeedClient.Machine().MachineV1alpha1().OpenStackMachineClasses(b.Shoot.SeedNamespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}

	for _, existingMachineClass := range existingMachineClasses.Items {
		if existingMachineClass.Spec.SecretRef == nil {
			return nil, nil, fmt.Errorf("could not find secret reference in class %s", existingMachineClass.Name)
		}

		secretNames.Insert(existingMachineClass.Spec.SecretRef.Name)
		classNames.Insert(existingMachineClass.Name)
	}

	return classNames, secretNames, nil
}

// CleanupMachineClasses deletes all machine classes which are not part of the provided list <existingMachineDeployments>.
func (b *OpenStackBotanist) CleanupMachineClasses(existingMachineDeployments operation.MachineDeployments) error {
	existingMachineClasses, err := b.K8sSeedClient.Machine().MachineV1alpha1().OpenStackMachineClasses(b.Shoot.SeedNamespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, existingMachineClass := range existingMachineClasses.Items {
		if !existingMachineDeployments.ContainsClass(existingMachineClass.Name) {
			if err := b.K8sSeedClient.Machine().MachineV1alpha1().OpenStackMachineClasses(b.Shoot.SeedNamespace).Delete(existingMachineClass.Name, &metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
	}

	return nil
}
