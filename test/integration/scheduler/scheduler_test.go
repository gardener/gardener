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

package scheduler

import (
	"context"
	"flag"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/scheduler/apis/config"
	. "github.com/gardener/gardener/test/integration/shoots"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/test/integration/framework"
)

var (
	kubeconfig             = flag.String("kubeconfig", "", "the path to the kubeconfig of Garden cluster that will be used for integration tests")
	shootTestYamlPath      = flag.String("shootpath", "", "the path to the shoot yaml that will be used for testing")
	testShootsPrefix       = flag.String("prefix", "", "prefix to use for test shoots")
	logLevel               = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
	schedulerTestNamespace = flag.String("scheduler-test-namespace", "", "the namespace where the shoot will be created")
)

const (
	WaitForCreateDeleteTimeout = 7200 * time.Second
	InitializationTimeout      = 600 * time.Second
	DumpStateTimeout           = 5 * time.Minute
)

func validateFlags() {
	if StringSet(*shootTestYamlPath) {
		if !FileExists(*shootTestYamlPath) {
			Fail("shoot yaml path is set but invalid")
		}
	}

	if !StringSet(*kubeconfig) {
		Fail("you need to specify the correct path for the kubeconfig")
	}

	if !FileExists(*kubeconfig) {
		Fail("kubeconfig path does not exist")
	}

	if !StringSet(*schedulerTestNamespace) {
		Fail("you need to specify the namespace where the shoot will be created")
	}
}

var _ = Describe("Scheduler testing", func() {
	var (
		gardenerTestOperation         *GardenerTestOperation
		schedulerGardenerTest         *SchedulerGardenerTest
		shoot                         *v1beta1.Shoot
		schedulerOperationsTestLogger *logrus.Logger
		cleanupNeeded                 bool
	)

	CBeforeSuite(func(ctx context.Context) {
		validateFlags()

		// parse shoot yaml into shoot object and generate random test names for shoots
		_, shootObject, err := CreateShootTestArtifacts(*shootTestYamlPath, *testShootsPrefix, false)
		Expect(err).To(BeNil())

		shoot = shootObject
		shoot.Spec.Cloud.Seed = nil
		schedulerOperationsTestLogger = logger.AddWriter(logger.NewLogger(*logLevel), GinkgoWriter)

		shootGardenerTest, err := NewShootGardenerTest(*kubeconfig, shoot, schedulerOperationsTestLogger)
		Expect(err).To(BeNil())

		schedulerGardenerTest, err = NewGardenSchedulerTest(ctx, shootGardenerTest, *kubeconfig)
		Expect(err).NotTo(HaveOccurred())
		schedulerGardenerTest.ShootGardenerTest.Shoot.Namespace = *schedulerTestNamespace

		gardenerTestOperation, err = NewGardenTestOperation(ctx, schedulerGardenerTest.ShootGardenerTest.GardenClient, schedulerOperationsTestLogger, nil)
		Expect(err).ToNot(HaveOccurred())
	}, InitializationTimeout)

	CAfterSuite(func(ctx context.Context) {
		if cleanupNeeded {
			// Finally we delete the shoot again
			schedulerOperationsTestLogger.Infof("Delete shoot %s", shoot.Name)
			err := schedulerGardenerTest.ShootGardenerTest.DeleteShoot(ctx)
			Expect(err).NotTo(HaveOccurred())
		}
	}, InitializationTimeout)

	CAfterEach(func(ctx context.Context) {
		gardenerTestOperation.AfterEach(ctx)
	}, DumpStateTimeout)

	// Only being executed if Scheduler is configured with SameRegion Strategy
	CIt("SameRegion Scheduling Strategy Test - should fail because no Seed in same region exists", func(ctx context.Context) {
		if schedulerGardenerTest.SchedulerConfiguration.Schedulers.Shoot.Strategy != config.SameRegion {
			schedulerOperationsTestLogger.Infof("Skipping Test, because Scheduler is not configured with strategy '%s' but with '%s'", config.SameRegion, schedulerGardenerTest.SchedulerConfiguration.Schedulers.Shoot.Strategy)
			return
		}

		// set shoot to a unsupportedRegion where no seed is deployed
		cloudProvider, unsupportedRegion, zoneNamesForUnsupportedRegion, err := schedulerGardenerTest.ChooseRegionAndZoneWithNoSeed()
		Expect(err).NotTo(HaveOccurred())
		schedulerGardenerTest.ShootGardenerTest.Shoot.Spec.Cloud.Region = *unsupportedRegion
		helper.SetZoneForShoot(schedulerGardenerTest.ShootGardenerTest.Shoot, *cloudProvider, zoneNamesForUnsupportedRegion)

		// First we create the target shoot.
		shootCreateDelete := context.WithValue(ctx, "name", "schedule shoot and delete the unschedulable shoot after")

		_, err = schedulerGardenerTest.CreateShoot(shootCreateDelete)
		Expect(err).NotTo(HaveOccurred())
		cleanupNeeded = true
		// expecting it to fail to schedule shoot and report in condition (api server sets)
		err = schedulerGardenerTest.WaitForShootToBeUnschedulable(ctx)
		Expect(err).NotTo(HaveOccurred())
	}, WaitForCreateDeleteTimeout)

	// Only being executed if Scheduler is configured with Minimal Distance Strategy
	// Tests if scheduling with MinimalDistance & actual creation of the shoot cluster in different region than the seed control works
	CIt("Minimal Distance Scheduling Strategy Test", func(ctx context.Context) {
		if schedulerGardenerTest.SchedulerConfiguration.Schedulers.Shoot.Strategy != config.MinimalDistance {
			schedulerOperationsTestLogger.Infof("Skipping Test, because Scheduler is not configured with strategy '%s' but with '%s'", config.MinimalDistance, schedulerGardenerTest.SchedulerConfiguration.Schedulers.Shoot.Strategy)
			return
		}
		// set shoot to a unsupportedRegion where no seed is deployed
		cloudProvider, unsupportedRegion, zoneNamesForUnsupportedRegion, err := schedulerGardenerTest.ChooseRegionAndZoneWithNoSeed()
		Expect(err).NotTo(HaveOccurred())
		schedulerGardenerTest.ShootGardenerTest.Shoot.Spec.Cloud.Region = *unsupportedRegion
		helper.SetZoneForShoot(schedulerGardenerTest.ShootGardenerTest.Shoot, *cloudProvider, zoneNamesForUnsupportedRegion)

		// First we create the target shoot.
		shootScheduling := context.WithValue(ctx, "name", "schedule shoot, create the shoot and delete the shoot after")

		_, err = schedulerGardenerTest.CreateShoot(shootScheduling)
		Expect(err).NotTo(HaveOccurred())
		cleanupNeeded = true
		// expecting it to to schedule shoot to seed
		err = schedulerGardenerTest.WaitForShootToBeScheduled(ctx)
		Expect(err).NotTo(HaveOccurred())
		//waiting for shoot to be successfully reconciled
		err = schedulerGardenerTest.ShootGardenerTest.WaitForShootToBeCreated(ctx)
		Expect(err).NotTo(HaveOccurred())
		schedulerOperationsTestLogger.Infof("Shoot %s was reconciled successfully!", shoot.Name)

		// We do apiserver scheduling tests here that rely on an existing shoot to avoid having to create another shoot

		By("Api Server ShootBindingStrategy test - wrong scheduling decision: seed does not exist")
		// create invalid Seed
		invalidSeed, err := schedulerGardenerTest.GenerateInvalidSeed()
		Expect(err).NotTo(HaveOccurred())

		// retrieve valid shoot
		alreadyScheduledShoot := &v1beta1.Shoot{}
		err = schedulerGardenerTest.ShootGardenerTest.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, alreadyScheduledShoot)
		Expect(err).NotTo(HaveOccurred())

		err = schedulerGardenerTest.ScheduleShoot(ctx, alreadyScheduledShoot, invalidSeed)
		Expect(err).To(HaveOccurred())

		// double check that invalid seed is not set
		currentShoot := &v1beta1.Shoot{}
		err = schedulerGardenerTest.ShootGardenerTest.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, currentShoot)
		Expect(err).NotTo(HaveOccurred())
		Expect(*currentShoot.Spec.Cloud.Seed).NotTo(Equal(invalidSeed.Name))

		By("Api Server ShootBindingStrategy test - wrong scheduling decision: already scheduled")

		// try to schedule a shoot that is already scheduled to another valid seed
		seed, err := schedulerGardenerTest.ChooseSeedWhereTestShootIsNotDeployed(currentShoot)

		if err != nil {
			schedulerOperationsTestLogger.Warnf("Test not executed: %v", err)
		} else {
			Expect(len(seed.Name)).NotTo(Equal(0))
			err = schedulerGardenerTest.ScheduleShoot(ctx, alreadyScheduledShoot, seed)
			Expect(err).To(HaveOccurred())
		}
	}, WaitForCreateDeleteTimeout)
})
