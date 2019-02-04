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
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// GetMachineClassInfo returns the name of the class kind, the plural of it and the name of the Helm chart which
// contains the machine class template.
func (b *AzureBotanist) GetMachineClassInfo() (classKind, classPlural, classChartName string) {
	classKind = "AzureMachineClass"
	classPlural = "azuremachineclasses"
	classChartName = "azure-machineclass"
	return
}

// GenerateMachineClassSecretData generates the secret data for the machine class secret (except the userData field
// which is computed elsewhere).
func (b *AzureBotanist) GenerateMachineClassSecretData() map[string][]byte {
	return map[string][]byte{
		machinev1alpha1.AzureClientID:       b.Shoot.Secret.Data[ClientID],
		machinev1alpha1.AzureClientSecret:   b.Shoot.Secret.Data[ClientSecret],
		machinev1alpha1.AzureSubscriptionID: b.Shoot.Secret.Data[SubscriptionID],
		machinev1alpha1.AzureTenantID:       b.Shoot.Secret.Data[TenantID],
	}
}

// GenerateMachineConfig generates the configuration values for the cloud-specific machine class Helm chart. It
// also generates a list of corresponding MachineDeployments. It returns the computed list of MachineClasses and
// MachineDeployments.
func (b *AzureBotanist) GenerateMachineConfig() ([]map[string]interface{}, operation.MachineDeployments, error) {
	var (
		resourceGroupName = "resourceGroupName"
		vnetName          = "vnetName"
		subnetName        = "subnetName"
		availabilitySetID = "availabilitySetID"
		outputVariables   = []string{resourceGroupName, vnetName, subnetName, availabilitySetID}
		workers           = b.Shoot.Info.Spec.Cloud.Azure.Workers

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
			secretData           = b.GenerateMachineClassSecretData()
		)

		machineDeployments = append(machineDeployments, operation.MachineDeployment{
			Name:           deploymentName,
			ClassName:      className,
			Minimum:        worker.AutoScalerMin,
			Maximum:        worker.AutoScalerMax,
			MaxSurge:       *worker.MaxSurge,
			MaxUnavailable: *worker.MaxUnavailable,
		})

		machineClassSpec["name"] = className
		machineClassSpec["secret"].(map[string]interface{})["clientID"] = string(secretData[machinev1alpha1.AzureClientID])
		machineClassSpec["secret"].(map[string]interface{})["clientSecret"] = string(secretData[machinev1alpha1.AzureClientSecret])
		machineClassSpec["secret"].(map[string]interface{})["subscriptionID"] = string(secretData[machinev1alpha1.AzureSubscriptionID])
		machineClassSpec["secret"].(map[string]interface{})["tenantID"] = string(secretData[machinev1alpha1.AzureTenantID])

		machineClasses = append(machineClasses, machineClassSpec)
	}

	return machineClasses, machineDeployments, nil
}

// ListMachineClasses returns two sets of strings whereas the first contains the names of all machine
// classes, and the second the names of all referenced secrets.
func (b *AzureBotanist) ListMachineClasses() (sets.String, sets.String, error) {
	var (
		classNames  = sets.NewString()
		secretNames = sets.NewString()
	)

	existingMachineClasses, err := b.K8sSeedClient.Machine().MachineV1alpha1().AzureMachineClasses(b.Shoot.SeedNamespace).List(metav1.ListOptions{})
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
func (b *AzureBotanist) CleanupMachineClasses(existingMachineDeployments operation.MachineDeployments) error {
	existingMachineClasses, err := b.K8sSeedClient.Machine().MachineV1alpha1().AzureMachineClasses(b.Shoot.SeedNamespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, existingMachineClass := range existingMachineClasses.Items {
		if !existingMachineDeployments.ContainsClass(existingMachineClass.Name) {
			if err := b.K8sSeedClient.Machine().MachineV1alpha1().AzureMachineClasses(b.Shoot.SeedNamespace).Delete(existingMachineClass.Name, &metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
	}

	return nil
}
