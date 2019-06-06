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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/test/integration/framework"
	"context"
)

// ApiServerBindingTestWrongSchedulerDecision executes Tests against the APIServer to test the behaviour needed for the Gardener Scheduler
// Requires the existence of an already scheduled shoot
// 1) wrong scheduling decision: Seed does not exist
// 2) wrong scheduling decision: double scheduling on already scheduled shoot
func ApiServerBindingTestWrongSchedulerDecision(ctx context.Context, schedulerGardenerTest *framework.SchedulerGardenerTest){
	// TEST CASE 1

	// create invalid seed
	invalidSeed, err := schedulerGardenerTest.GenerateInvalidSeed()
	if err != nil {
		testLogger.Fatalf("Failed to generate an invalid seed: %s", err.Error())
	}

	// retrieve valid shoot - retrieve from cluster to make sure it exists (Apiserver does that too)
	alreadyScheduledShoot := &v1beta1.Shoot{}
	shoot := schedulerGardenerTest.ShootGardenerTest.Shoot
	err = schedulerGardenerTest.ShootGardenerTest.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, alreadyScheduledShoot)
	if err != nil {
		testLogger.Fatalf("Failed to retrieve shoot from cluster: %s: %s", shoot.Name, err.Error())
	}

	err = schedulerGardenerTest.ScheduleShoot(ctx, alreadyScheduledShoot, invalidSeed)
	if err == nil {
		testLogger.Fatalf("Expected the Api Server to return a BadRequest Error when creating a shoot binding for a shoot (%s) using an invalid seed (%s)", shoot.Name, invalidSeed.Name)
	}

	// double check that invalid seed is not set
	currentShoot := &v1beta1.Shoot{}
	err = schedulerGardenerTest.ShootGardenerTest.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, currentShoot)
	if err != nil {
		testLogger.Fatalf("Failed to retrieve shoot from cluster: %s: %s", shoot.Name, err.Error())
	}

	if (*currentShoot.Spec.Cloud.Seed == invalidSeed.Name){
		testLogger.Fatalf("Shoot (%s) got updated with an invalid seed (%s). This should never happen.", shoot.Name, invalidSeed.Name)
	}

	// TEST CASE 2

	// try to schedule a shoot that is already scheduled to another valid seed
	seed, err := schedulerGardenerTest.ChooseSeedWhereTestShootIsNotDeployed(currentShoot)
	if err != nil {
		testLogger.Warnf("Test not executed: %v", err)
	} else {
		if len(seed.Name) == 0 {
			testLogger.Fatalf("Failed to retrieve a valid seed from the current cluster")
		}
		err = schedulerGardenerTest.ScheduleShoot(ctx, alreadyScheduledShoot, seed)
		if err == nil {
			testLogger.Fatalf("Expected the Api Server to return a BadRequest Error when creating a shoot binding for a shoot (%s) that is already scheduled to a seed", shoot.Name)
		}
	}
}