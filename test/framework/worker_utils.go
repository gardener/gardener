// Copyright 2019 Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
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

package framework

import (
	"fmt"
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/utils"
)

// AddWorkerForName adds a valid worker to the shoot for the given machine image name. Returns an error if the machine image cannot be found in the CloudProfile.
func AddWorkerForName(shoot *gardencorev1beta1.Shoot, cloudProfile *gardencorev1beta1.CloudProfile, machineImageName *string, workerZone *string) error {
	found, image, err := helper.DetermineMachineImageForName(cloudProfile, *machineImageName)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("could not find machine image '%s' in CloudProfile '%s'", *machineImageName, cloudProfile.Name)
	}

	return AddWorker(shoot, cloudProfile, image, workerZone)
}

// SetupShootWorker prepares the Shoot with one worker with provider specific volume. Clears the currently configured workers.
func SetupShootWorker(shoot *gardencorev1beta1.Shoot, cloudProfile *gardencorev1beta1.CloudProfile, workerZone *string) error {
	if len(cloudProfile.Spec.MachineImages) < 1 {
		return fmt.Errorf("at least one different machine image has to be defined in the CloudProfile")
	}

	// clear current workers
	shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{}

	if err := AddWorker(shoot, cloudProfile, cloudProfile.Spec.MachineImages[0], workerZone); err != nil {
		return err
	}
	return nil
}

// AddWorker adds a valid default worker to the shoot for the given machineImage and CloudProfile.
func AddWorker(shoot *gardencorev1beta1.Shoot, cloudProfile *gardencorev1beta1.CloudProfile, machineImage gardencorev1beta1.MachineImage, workerZone *string) error {
	_, shootMachineImage, err := helper.GetShootMachineImageFromLatestMachineImageVersion(machineImage)
	if err != nil {
		return err
	}

	if len(cloudProfile.Spec.MachineTypes) == 0 {
		return fmt.Errorf("no MachineTypes configured in the Cloudprofile '%s'", cloudProfile.Name)
	}
	machineType := cloudProfile.Spec.MachineTypes[0]

	workerName, err := generateRandomWorkerName(fmt.Sprintf("%s-", shootMachineImage.Name))
	if err != nil {
		return err
	}

	shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, gardencorev1beta1.Worker{
		Name:    workerName,
		Maximum: 2,
		Minimum: 2,
		Machine: gardencorev1beta1.Machine{
			Type:  machineType.Name,
			Image: &shootMachineImage,
		},
	})

	if machineType.Storage == nil {
		if len(cloudProfile.Spec.VolumeTypes) == 0 {
			return fmt.Errorf("no VolumeTypes configured in the Cloudprofile '%s'", cloudProfile.Name)
		}
		shoot.Spec.Provider.Workers[0].Volume = &gardencorev1beta1.Volume{
			Type:       &cloudProfile.Spec.VolumeTypes[0].Name,
			VolumeSize: "35Gi",
		}
	}

	if workerZone != nil && len(*workerZone) > 0 {
		// using one zone as default
		shoot.Spec.Provider.Workers[0].Zones = []string{*workerZone}
	}

	return nil
}

func generateRandomWorkerName(prefix string) (string, error) {
	var length int
	remainingCharacters := 15 - len(prefix)
	if remainingCharacters > 0 {
		length = remainingCharacters
	} else {
		prefix = WorkerNamePrefix
		length = 15 - len(WorkerNamePrefix)
	}

	randomString, err := utils.GenerateRandomString(length)
	if err != nil {
		return "", err
	}

	return prefix + strings.ToLower(randomString), nil
}
