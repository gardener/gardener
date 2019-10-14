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

/**
	Overview
		- Tests the Gardener Scheduler

	Prerequisites
		- The Gardener-Scheduler is running

	BeforeSuite
		- Parse valid Shoot from example folder and flags. Remove the Spec.SeedName.
		- If running in TestMachinery: Scale down the GardenerController Manager

	AfterSuite
		- Delete Shoot
        - If running in TestMachinery: Scale up the GardenerController Manager

	Test: SameRegion Scheduling Strategy Test
		1) Create Shoot in region where no Seed exists. (e.g Shoot in eu-west-1 and only Seed exists in us-east-1)
		   Expected Output
			 - should fail because no Seed in same region exists1)
	Test: Minimal Distance Scheduling Strategy Test
		1) Create Shoot in region where no Seed exists. (e.g Shoot in eu-west-1 and only Seed exists in us-east-1)
		   Expected Output
			 - should successfully schedule to Seed in region with minimal distance
	Test: Api Server ShootBindingStrategy test
		1) Request APiServer to schedule shoot to non-existing seed
		   Expected Output
			 - Error from ApiServer
		2) Request APiServer to schedule shoot that is already scheduled to another seed
		   Expected Output
			 - Error from ApiServer
 **/

package scheduler

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/logger"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/scheduler/apis/config"
	. "github.com/gardener/gardener/test/integration/shoots"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"

	. "github.com/gardener/gardener/test/integration/framework"
)

var (
	kubeconfig             = flag.String("kubecfg", "", "the path to the kubeconfig of Garden cluster that will be used for integration tests")
	testShootsPrefix       = flag.String("prefix", "", "prefix to use for test shoots")
	cloudprofile           = flag.String("cloud-profile", "", "cloudprofile to use for the shoot")
	logLevel               = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
	schedulerTestNamespace = flag.String("scheduler-test-namespace", "", "the namespace where the shoot will be created")
	secretBinding          = flag.String("secret-binding", "", "the secretBinding for the provider account of the shoot")
	providerType           = flag.String("provider-type", "", "the type of the cloud provider where the shoot is deployed to. e.g gcp, aws,azure,alicloud")
	shootK8sVersion        = flag.String("k8s-version", "", "kubernetes version to use for the shoot")
	testMachineryRun       = flag.Bool("test-machinery-run", false, "indicates whether the test is being executed by the test machinery or locally")
	projectNamespace       = flag.String("project-namespace", "", "project namespace where the shoot will be created")
	workerZone             = flag.String("worker-zone", "", "zone to use for every worker of the shoot.")

	// ProviderConfigs
	infrastructureProviderConfig = flag.String("infrastructure-provider-config-filepath", "", "filepath to the provider specific infrastructure config")

	schedulerGardenerTest         *SchedulerGardenerTest
	gardenerSchedulerReplicaCount *int32
)

const (
	WaitForCreateDeleteTimeout = 600 * time.Second
	InitializationTimeout      = 600 * time.Second
	DumpStateTimeout           = 5 * time.Minute
	// timeouts
	setupContextTimeout = time.Minute * 2
	restoreCtxTimeout   = time.Minute * 2
)

func validateFlags() {
	if !StringSet(*kubeconfig) {
		Fail("you need to specify a path to the Kubeconfig of the Garden cluster")
	}

	if !FileExists(*kubeconfig) {
		Fail("path to the Kubeconfig of the Garden cluster does not exist")
	}

	if !StringSet(*schedulerTestNamespace) {
		Fail("you need to specify the namespace where the shoot will be created")
	}

	if !StringSet(*logLevel) {
		level := "debug"
		logLevel = &level
	}

	if !StringSet(*providerType) {
		Fail("you need to specify provider type of the shoot")
	}

	if !StringSet(*infrastructureProviderConfig) {
		Fail(fmt.Sprintf("you need to specify the filepath to the infrastructureProviderConfig for the provider '%s'", *providerType))
	}

	if !FileExists(*infrastructureProviderConfig) {
		Fail("path to the infrastructureProviderConfig of the Shoot is invalid")
	}

}

var _ = Describe("Scheduler testing", func() {
	var (
		gardenerTestOperation         *GardenerTestOperation
		shoot                         *gardencorev1alpha1.Shoot
		schedulerOperationsTestLogger *logrus.Logger
		cleanupNeeded                 bool
		shootYamlPath                 = "/example/90-shoot.yaml"
	)

	CBeforeSuite(func(ctx context.Context) {
		schedulerOperationsTestLogger = logger.AddWriter(logger.NewLogger(*logLevel), GinkgoWriter)
		validateFlags()

		// if running in test machinery, test will be executed from root of the project
		if !FileExists(fmt.Sprintf(".%s", shootYamlPath)) {
			// locally, we need find the example shoot
			shootYamlPath = GetProjectRootPath() + shootYamlPath
			Expect(FileExists(shootYamlPath)).To(Equal(true))
		}

		// parse shoot yaml into shoot object and generate random test names for shoots
		_, shootObject, err := CreateShootTestArtifacts(shootYamlPath, testShootsPrefix, projectNamespace, nil, cloudprofile, secretBinding, providerType, shootK8sVersion, nil, true, true)
		Expect(err).To(BeNil())

		shoot = shootObject
		shoot.Spec.SeedName = nil

		// parse Infrastructure config
		infrastructureProviderConfig, err := ParseFileAsProviderConfig(*infrastructureProviderConfig)
		Expect(err).To(BeNil())
		shoot.Spec.Provider.InfrastructureConfig = infrastructureProviderConfig

		// set other provider configs to nil as we do not need them for shoot creation (without reconciliation)
		shoot.Spec.Provider.ControlPlaneConfig = nil
		shoot.Spec.Extensions = nil

		shootGardenerTest, err := NewShootGardenerTest(*kubeconfig, shoot, schedulerOperationsTestLogger)
		Expect(err).To(BeNil())

		shootGardenerTest.SetupShootWorker(workerZone)
		Expect(len(shootGardenerTest.Shoot.Spec.Provider.Workers)).Should(BeNumerically(">=", 1))

		schedulerGardenerTest, err = NewGardenSchedulerTest(ctx, shootGardenerTest, *kubeconfig)
		Expect(err).NotTo(HaveOccurred())
		schedulerGardenerTest.ShootGardenerTest.Shoot.Namespace = *schedulerTestNamespace

		if testMachineryRun != nil && *testMachineryRun {
			schedulerOperationsTestLogger.Info("Running in test Machinery")
			zero := int32(0)
			replicas, err := ScaleGardenerControllerManager(setupContextTimeout, schedulerGardenerTest.ShootGardenerTest.GardenClient.Client(), &zero)
			Expect(err).To(BeNil())
			gardenerSchedulerReplicaCount = replicas
			schedulerOperationsTestLogger.Info("Environment for test-machinery run is prepared")
		}

		gardenerTestOperation, err = NewGardenTestOperationWithShoot(ctx, schedulerGardenerTest.ShootGardenerTest.GardenClient, schedulerOperationsTestLogger, nil)
		Expect(err).ToNot(HaveOccurred())
	}, InitializationTimeout)

	CAfterSuite(func(ctx context.Context) {
		if cleanupNeeded {
			// Finally we delete the shoot again
			schedulerOperationsTestLogger.Infof("Delete shoot %s", shoot.Name)
			err := schedulerGardenerTest.ShootGardenerTest.DeleteShoot(ctx)
			Expect(err).NotTo(HaveOccurred())
		}
		if testMachineryRun != nil && *testMachineryRun {
			// Scale up ControllerManager again to restore state before this test.
			_, err := ScaleGardenerControllerManager(restoreCtxTimeout, schedulerGardenerTest.ShootGardenerTest.GardenClient.Client(), gardenerSchedulerReplicaCount)
			Expect(err).NotTo(HaveOccurred())
			schedulerOperationsTestLogger.Infof("Environment is restored")
		}
		// wait until shoot is deleted
		if cleanupNeeded {
			err := schedulerGardenerTest.ShootGardenerTest.WaitForShootToBeDeleted(ctx)
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
		unsupportedRegion, err := schedulerGardenerTest.ChooseRegionAndZoneWithNoSeed()
		Expect(err).NotTo(HaveOccurred())
		schedulerGardenerTest.ShootGardenerTest.Shoot.Spec.Region = unsupportedRegion.Name

		// First we create the target shoot.
		shootCreateDeleteContext := context.WithValue(ctx, "name", "schedule shoot and delete the unschedulable shoot after")

		_, err = schedulerGardenerTest.CreateShoot(shootCreateDeleteContext)
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
		unsupportedRegion, err := schedulerGardenerTest.ChooseRegionAndZoneWithNoSeed()
		Expect(err).NotTo(HaveOccurred())
		schedulerGardenerTest.ShootGardenerTest.Shoot.Spec.Region = unsupportedRegion.Name
		// setZones(unsupportedRegion)

		// First we create the target shoot.
		shootScheduling := context.WithValue(ctx, "name", "schedule shoot, create the shoot and delete the shoot after")

		_, err = schedulerGardenerTest.CreateShoot(shootScheduling)
		Expect(err).NotTo(HaveOccurred())
		cleanupNeeded = true
		// expecting it to to schedule shoot to seed
		err = schedulerGardenerTest.WaitForShootToBeScheduled(ctx)
		Expect(err).NotTo(HaveOccurred())

		// We do apiserver scheduling tests here that rely on an existing shoot to avoid having to create another shoot
		By("Api Server ShootBindingStrategy test - wrong scheduling decision: seed does not exist")
		// create invalid Seed
		invalidSeed, err := schedulerGardenerTest.GenerateInvalidSeed()
		Expect(err).NotTo(HaveOccurred())

		// retrieve valid shoot
		alreadyScheduledShoot := &gardencorev1alpha1.Shoot{}
		err = schedulerGardenerTest.ShootGardenerTest.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, alreadyScheduledShoot)
		Expect(err).NotTo(HaveOccurred())

		err = schedulerGardenerTest.ScheduleShoot(ctx, alreadyScheduledShoot, invalidSeed)
		Expect(err).To(HaveOccurred())

		// double check that invalid seed is not set
		currentShoot := &gardencorev1alpha1.Shoot{}
		err = schedulerGardenerTest.ShootGardenerTest.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, currentShoot)
		Expect(err).NotTo(HaveOccurred())
		Expect(*currentShoot.Spec.SeedName).NotTo(Equal(invalidSeed.Name))

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
