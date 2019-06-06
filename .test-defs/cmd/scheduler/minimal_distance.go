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
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/test/integration/framework"
)

// MinimalDistanceTest makes sure that a shoot can be created in a different region than itself - given the scheduler is configured with MinimalDistance Strategy
func MinimalDistanceTest(ctx context.Context, schedulerGardenerTest *framework.SchedulerGardenerTest) {
	// set shoot to a unsupportedRegion where no seed is deployed
	cloudProvider, unsupportedRegion, zoneNamesForUnsupportedRegion, err := schedulerGardenerTest.ChooseRegionAndZoneWithNoSeed()
	if err != nil {
		testLogger.Fatalf("Failed to choose a region from the cluster that does not have a seed: %s", err.Error())
	}
	schedulerGardenerTest.ShootGardenerTest.Shoot.Spec.Cloud.Region = *unsupportedRegion
	helper.SetZoneForShoot(schedulerGardenerTest.ShootGardenerTest.Shoot, *cloudProvider, zoneNamesForUnsupportedRegion)

	testLogger.Infof("Create shoot %s in namespace %s with unsupportedRegion %s", shootName, projectNamespace, unsupportedRegion)
	_, err = schedulerGardenerTest.CreateShoot(ctx)
	if err != nil {
		testLogger.Fatalf("Cannot create shoot %s: %s", shootName, err.Error())
	}
	testLogger.Infof("Successfully created shoot %s", shootName)
	defer schedulerGardenerTest.ShootGardenerTest.DeleteShoot(ctx)

	// expecting it to fail to schedule shoot and report in condition (api server sets)
	err = schedulerGardenerTest.WaitForShootToBeScheduled(ctx)
	if err != nil {
		testLogger.Fatalf("Failed to wait for shoot to be scheduled to a seed: %s: %s", shootName, err.Error())
	}

	//waiting for shoot to be successfully reconciled
	err = schedulerGardenerTest.ShootGardenerTest.WaitForShootToBeCreated(ctx)
	if err != nil {
		testLogger.Fatalf("Failed to wait for shoot to be reconciled successfully: %s: %s", shootName, err.Error())
	}
	testLogger.Infof("Shoot %s was reconciled successfully!", shootName)
}
