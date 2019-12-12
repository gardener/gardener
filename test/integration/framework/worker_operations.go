// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package framework

import (
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// NewWorkerGardenerTest creates a new NewWorkerGardenerTest
func NewWorkerGardenerTest(shootGardenTest *ShootGardenerTest) (*WorkerGardenerTest, error) {
	return &WorkerGardenerTest{
		ShootGardenerTest: shootGardenTest,
	}, nil
}

// SetupShootWorkers prepares the Shoot with multiple workers. Only supporting one optional zone.
func (s *WorkerGardenerTest) SetupShootWorkers(shootMachineImageName *string, shootMachineImageName2 *string, workerZone *string) error {
	if len(s.CloudProfile.Spec.MachineImages) < 2 {
		return fmt.Errorf("this integration tests needs at least two different machine images to be defined in the CloudProfile")
	}

	// clear current workers
	s.ShootGardenerTest.Shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{}

	// determine two different machine image names from the CloudProfile
	if shootMachineImageName == nil || len(*shootMachineImageName) == 0 || shootMachineImageName2 == nil || len(*shootMachineImageName2) == 0 {
		for imageNumber := 0; imageNumber < 2; imageNumber++ {
			if err := AddWorker(s.ShootGardenerTest.Shoot, s.ShootGardenerTest.CloudProfile, s.CloudProfile.Spec.MachineImages[imageNumber], workerZone); err != nil {
				return err
			}
		}
	} else {
		// set the latest version for the provided image names
		err := AddWorkerForName(s.ShootGardenerTest.Shoot, s.ShootGardenerTest.CloudProfile, shootMachineImageName, workerZone)
		if err != nil {
			return err
		}

		err = AddWorkerForName(s.ShootGardenerTest.Shoot, s.ShootGardenerTest.CloudProfile, shootMachineImageName2, workerZone)
		if err != nil {
			return err
		}
	}
	return nil
}
