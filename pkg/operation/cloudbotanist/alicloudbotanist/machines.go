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

package alicloudbotanist

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
func (b *AlicloudBotanist) GetMachineClassInfo() (classKind, classPlural, classChartName string) {
	classKind = "AlicloudMachineClass"
	classPlural = "alicloudmachineclasses"
	classChartName = "alicloud-machineclass"

	return
}

// GenerateMachineClassSecretData generates the secret data for the machine class secret (except the userData field
// which is computed elsewhere).
func (b *AlicloudBotanist) GenerateMachineClassSecretData() map[string][]byte {
	return map[string][]byte{
		machinev1alpha1.AlicloudAccessKeyID:     b.Shoot.Secret.Data[AccessKeyID],
		machinev1alpha1.AlicloudAccessKeySecret: b.Shoot.Secret.Data[AccessKeySecret],
	}
}

// GenerateMachineConfig generates the configuration values for the cloud-specific machine class Helm chart. It
// also generates a list of corresponding MachineDeployments. The provided worker groups will be distributed over
// the desired availability zones. It returns the computed list of MachineClasses and MachineDeployments.
func (b *AlicloudBotanist) GenerateMachineConfig() ([]map[string]interface{}, operation.MachineDeployments, error) {
	var (
		securityGroupID     = "sg_id"
		keyPairName         = "key_pair_name"
		outputVariables     = []string{securityGroupID, keyPairName}
		workers             = b.Shoot.Info.Spec.Cloud.Alicloud.Workers
		zones               = b.Shoot.Info.Spec.Cloud.Alicloud.Zones
		machineDeployments  = operation.MachineDeployments{}
		machineClasses      = []map[string]interface{}{}
		secretData          = b.GenerateMachineClassSecretData()
		tfOutputNameVswitch = func(zoneIndex int) string {
			return fmt.Sprintf("vswitch_id_z%d", zoneIndex)
		}
	)
	for zoneIndex := range zones {
		outputVariables = append(outputVariables, tfOutputNameVswitch(zoneIndex))
	}

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
				"imageID":         b.Shoot.Info.Spec.Cloud.Alicloud.MachineImage.ID,
				"instanceType":    worker.MachineType,
				"region":          b.Shoot.Info.Spec.Cloud.Region,
				"zoneID":          zone,
				"securityGroupID": stateVariables[securityGroupID],
				"vSwitchID":       stateVariables[tfOutputNameVswitch(zoneIndex)],
				"systemDisk": map[string]interface{}{
					"category": worker.VolumeType,
					"size":     common.DiskSize(worker.VolumeSize),
				},
				"instanceChargeType":      "PostPaid",
				"internetChargeType":      "PayByTraffic",
				"internetMaxBandwidthIn":  5,
				"internetMaxBandwidthOut": 5,
				"spotStrategy":            "NoSpot",
				"tags": map[string]string{
					fmt.Sprintf("kubernetes.io/cluster/%s", b.Shoot.SeedNamespace):     "1",
					fmt.Sprintf("kubernetes.io/role/worker/%s", b.Shoot.SeedNamespace): "1",
				},
				"keyPairName": stateVariables[keyPairName],
			}

			var (
				machineClassSpecHash = common.MachineClassHash(machineClassSpec, b.Shoot.KubernetesMajorMinorVersion)
				deploymentName       = fmt.Sprintf("%s-%s-%s", b.Shoot.SeedNamespace, worker.Name, zone)
				className            = fmt.Sprintf("%s-%s", deploymentName, machineClassSpecHash)
			)

			machineDeployments = append(machineDeployments, operation.MachineDeployment{
				Name:      deploymentName,
				ClassName: className,
				Minimum:   worker.AutoScalerMin,
				Maximum:   worker.AutoScalerMax,
			})

			machineClassSpec["name"] = className
			machineClassSpec["secret"] = map[string]interface{}{
				UserData:        cloudConfig.FileContent("cloud-config.yaml"),
				AccessKeyID:     secretData[machinev1alpha1.AlicloudAccessKeyID],
				AccessKeySecret: secretData[machinev1alpha1.AlicloudAccessKeySecret],
			}

			machineClasses = append(machineClasses, machineClassSpec)

		}
	}

	return machineClasses, machineDeployments, nil
}

// ListMachineClasses returns two sets of strings whereas the first contains the names of all machine
// classes, and the second the names of all referenced secrets.
func (b *AlicloudBotanist) ListMachineClasses() (sets.String, sets.String, error) {
	var (
		classNames  = sets.NewString()
		secretNames = sets.NewString()
	)

	existingMachineClasses, err := b.K8sSeedClient.Machine().MachineV1alpha1().AlicloudMachineClasses(b.Shoot.SeedNamespace).List(metav1.ListOptions{})
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
func (b *AlicloudBotanist) CleanupMachineClasses(existingMachineDeployments operation.MachineDeployments) error {
	existingMachineClasses, err := b.K8sSeedClient.Machine().MachineV1alpha1().AlicloudMachineClasses(b.Shoot.SeedNamespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, existingMachineClass := range existingMachineClasses.Items {
		if !existingMachineDeployments.ContainsClass(existingMachineClass.Name) {
			if err := b.K8sSeedClient.Machine().MachineV1alpha1().AlicloudMachineClasses(b.Shoot.SeedNamespace).Delete(existingMachineClass.Name, &metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
	}

	return nil
}
