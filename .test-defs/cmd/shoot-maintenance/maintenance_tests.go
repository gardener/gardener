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
// limitations under the License.

package main

import (
	"context"
	"time"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/test/integration/framework"
)
var (
	falseVar = false
	trueVar  = true
)

func testMachineImageMaintenance(ctx context.Context, shootGardenerTest *framework.ShootGardenerTest, shootMaintenanceTest *framework.ShootMaintenanceTest, shoot *gardenv1beta1.Shoot) {
	// TEST CASE: AutoUpdate.MachineImageVersion == false && expirationDate does not apply -> shoot machineImage must not be updated in maintenance time
	integrationTestShoot, err := shootGardenerTest.GetShoot(ctx)
	if err != nil {
		testLogger.Fatalf("Failed retrieve the test shoot: %s", err.Error())
	}

	// set test specific shoot settings
	integrationTestShoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = &falseVar
	integrationTestShoot.Annotations[common.ShootOperation] = common.ShootOperationMaintain

	// update integration test shoot
	err = shootMaintenanceTest.TryUpdateShootForMaintenance(ctx, integrationTestShoot, false, nil)
	if err != nil {
		testLogger.Fatalf("Failed to update shoot for maintenance: %s", err.Error())
	}

	err = shootMaintenanceTest.WaitForExpectedMaintenance(ctx, testMachineImage, shootMaintenanceTest.CloudProvider, false, time.Now().Add(time.Minute*1))
	if err != nil {
		testLogger.Fatalf("Failed to wait for expected machine image maintenance on shoot: %s", err.Error())
	}

	// TEST CASE: AutoUpdate.MachineImageVersion == true && expirationDate does not apply -> shoot machineImage must be updated in maintenance time

	// set test specific shoot settings
	integrationTestShoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = &trueVar
	integrationTestShoot.Annotations[common.ShootOperation] = common.ShootOperationMaintain

	// update integration test shoot - set maintain now annotation & autoupdate == true
	err = shootMaintenanceTest.TryUpdateShootForMaintenance(ctx, integrationTestShoot, false, nil)
	if err != nil {
		testLogger.Fatalf("Failed to update shoot for maintenance: %s", err.Error())
	}

	err = shootMaintenanceTest.WaitForExpectedMaintenance(ctx, shootMaintenanceTest.ShootMachineImage, shootMaintenanceTest.CloudProvider, true, time.Now().Add(time.Minute*1))
	if err != nil {
		testLogger.Fatalf("Failed to wait for expected machine image maintenance on shoot: %s", err.Error())
	}

	// TEST CASE: AutoUpdate.MachineImageVersion == default && expirationDate does not apply -> shoot machineImage must be updated in maintenance time

	// set test specific shoot settings
	integrationTestShoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = nil
	integrationTestShoot.Annotations[common.ShootOperation] = common.ShootOperationMaintain

	// reset machine image from latest version to dummy version
	updateImage := v1beta1helper.UpdateMachineImage(shootMaintenanceTest.CloudProvider, &testMachineImage)
	if err != nil {
		testLogger.Fatalf("Failed to update machine image: %s", err.Error())
	}

	// update integration test shoot - downgrade image again & set maintain now  annotation & autoupdate == nil (default)
	err = shootMaintenanceTest.TryUpdateShootForMaintenance(ctx, integrationTestShoot, true, updateImage)
	if err != nil {
		testLogger.Fatalf("Failed to update shoot for maintenance: %s", err.Error())
	}

	err = shootMaintenanceTest.WaitForExpectedMaintenance(ctx, shootMaintenanceTest.ShootMachineImage, shootMaintenanceTest.CloudProvider, true, time.Now().Add(time.Minute*1))
	if err != nil {
		testLogger.Fatalf("Failed to wait for expected machine image maintenance on shoot: %s", err.Error())
	}

	// TEST CASE: AutoUpdate.MachineImageVersion == false && expirationDate does apply -> shoot machineImage must be updated in maintenance time
	// modify cloud profile for test
	err = shootMaintenanceTest.TryUpdateCloudProfileForMaintenance(ctx, shoot, testMachineImage)
	if err != nil {
		testLogger.Fatalf("Failed to update CloudProfile for maintenance: %s", err.Error())
	}

	// set test specific shoot settings
	integrationTestShoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = &falseVar
	integrationTestShoot.Annotations[common.ShootOperation] = common.ShootOperationMaintain

	// reset machine image from latest version to dummy version
	updateImage = v1beta1helper.UpdateMachineImage(shootMaintenanceTest.CloudProvider, &testMachineImage)
	if err != nil {
		testLogger.Fatalf("Failed to update machine image: %s", err.Error())
	}

	// update integration test shoot - downgrade image again & set maintain now  annotation & autoupdate == nil (default)
	err = shootMaintenanceTest.TryUpdateShootForMaintenance(ctx, integrationTestShoot, true, updateImage)
	if err != nil {
		testLogger.Fatalf("Failed to update shoot for maintenance: %s", err.Error())
	}

	err = shootMaintenanceTest.WaitForExpectedMaintenance(ctx, shootMaintenanceTest.ShootMachineImage, shootMaintenanceTest.CloudProvider, true, time.Now().Add(time.Minute*1))
	if err != nil {
		testLogger.Fatalf("Failed to wait for expected machine image maintenance on shoot: %s", err.Error())
	}
}