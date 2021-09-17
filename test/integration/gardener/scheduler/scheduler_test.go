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

	"github.com/gardener/gardener/pkg/utils"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/scheduler/apis/config"
	schedulerconfigv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
	"github.com/gardener/gardener/test/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TODO: this test is currently not executed, because it scales down gardener-controller-manager (probably from times
//  before the gardenlet). It should be refactor into an envtest-style integration test, so that we can enable it again.

var (
	hostKubeconfig   = flag.String("host-kubeconfig", "", "the path to the kubeconfig  of the cluster that hosts the gardener controlplane")
	testMachineryRun = flag.Bool("test-machinery-run", false, "indicates whether the test is being executed by the test machinery or locally")

	gardenerSchedulerReplicaCount *int32
)

const (
	WaitForCreateDeleteTimeout = 600 * time.Second
	InitializationTimeout      = 600 * time.Second
)

func init() {
	framework.RegisterShootCreationFrameworkFlags()
}

type name string

const nameKey name = "name"

var _ = Describe("Scheduler testing", func() {

	f := framework.NewShootCreationFramework(&framework.ShootCreationConfig{
		GardenerConfig: &framework.GardenerConfig{
			CommonConfig: &framework.CommonConfig{
				ResourceDir: "../../framework/resources",
			},
		},
	})
	var (
		hostClusterClient      kubernetes.Interface
		cloudprofile           *gardencorev1beta1.CloudProfile
		seeds                  []gardencorev1beta1.Seed
		schedulerConfiguration *config.SchedulerConfiguration
		cleanupNeeded          bool
	)

	framework.CBeforeSuite(func(ctx context.Context) {
		framework.ExpectNoError(f.InitializeShootWithFlags(ctx))
		f.Shoot.Spec.SeedName = nil

		// set other provider configs to nil as we do not need them for shoot creation (without reconciliation)
		f.Shoot.Spec.Provider.ControlPlaneConfig = nil
		f.Shoot.Spec.Extensions = nil

		var err error
		hostClusterClient, err = kubernetes.NewClientFromFile("", *hostKubeconfig, kubernetes.WithClientOptions(
			client.Options{
				Scheme: kubernetes.ShootScheme,
			}),
		)
		framework.ExpectNoError(err)

		schedulerConfigurationConfigMap := &corev1.ConfigMap{}
		err = hostClusterClient.Client().Get(ctx, client.ObjectKey{Namespace: schedulerconfigv1alpha1.SchedulerDefaultConfigurationConfigMapNamespace, Name: schedulerconfigv1alpha1.SchedulerDefaultConfigurationConfigMapName}, schedulerConfigurationConfigMap)
		framework.ExpectNoError(err)

		schedulerConfiguration, err = framework.ParseSchedulerConfiguration(schedulerConfigurationConfigMap)
		framework.ExpectNoError(err)

		cloudprofile, err = f.GetCloudProfile(ctx, f.Shoot.Spec.CloudProfileName)
		framework.ExpectNoError(err)

		seeds, err = f.GetSeeds(ctx)
		framework.ExpectNoError(err)

		if testMachineryRun != nil && *testMachineryRun {
			f.Logger.Info("Running in test Machinery")
			replicas, err := framework.ScaleGardenerControllerManager(ctx, f.GardenClient.Client(), pointer.Int32(0))
			Expect(err).To(BeNil())
			gardenerSchedulerReplicaCount = replicas
			f.Logger.Info("Environment for test-machinery run is prepared")
		}
	}, InitializationTimeout)

	framework.CAfterSuite(func(ctx context.Context) {
		if cleanupNeeded {
			// Finally we delete the shoot again
			f.Logger.Infof("Delete shoot %s", f.Shoot.Name)
			err := f.DeleteShoot(ctx, f.Shoot)
			framework.ExpectNoError(err)
		}
		if testMachineryRun != nil && *testMachineryRun {
			// Scale up ControllerManager again to restore state before this test.
			_, err := framework.ScaleGardenerControllerManager(ctx, f.GardenClient.Client(), gardenerSchedulerReplicaCount)
			framework.ExpectNoError(err)
			f.Logger.Infof("Environment is restored")
		}
		// wait until shoot is deleted
		if cleanupNeeded {
			err := f.WaitForShootToBeDeleted(ctx, f.Shoot)
			framework.ExpectNoError(err)
		}
	}, InitializationTimeout)

	// Only being executed if Scheduler is configured with SameRegion Strategy
	framework.CIt("SameRegion Scheduling Strategy Test - should fail because no Seed in same region exists", func(ctx context.Context) {
		if schedulerConfiguration.Schedulers.Shoot.Strategy != config.SameRegion {
			f.Logger.Infof("Skipping Test, because Scheduler is not configured with strategy '%s' but with '%s'", config.SameRegion, schedulerConfiguration.Schedulers.Shoot.Strategy)
			return
		}

		// set shoot to a unsupportedRegion where no seed is deployed
		unsupportedRegion, err := ChooseRegionAndZoneWithNoSeed(cloudprofile.Spec.Regions, seeds)
		Expect(err).NotTo(HaveOccurred())
		f.Shoot.Spec.Region = unsupportedRegion.Name

		// First we create the target shoot.
		shootCreateDeleteContext := context.WithValue(ctx, nameKey, "schedule shoot and delete the unschedulable shoot after")

		err = f.GardenerFramework.CreateShoot(shootCreateDeleteContext, f.Shoot)
		framework.ExpectNoError(err)
		cleanupNeeded = true
		// expecting it to fail to schedule shoot and report in condition (api server sets)
		err = f.WaitForShootToBeUnschedulable(ctx, f.Shoot)
		framework.ExpectNoError(err)
	}, WaitForCreateDeleteTimeout)

	// Only being executed if Scheduler is configured with Minimal Distance Strategy
	// Tests if scheduling with MinimalDistance & actual creation of the shoot cluster in different region than the seed control works
	framework.CIt("Minimal Distance Scheduling Strategy Test", func(ctx context.Context) {
		if schedulerConfiguration.Schedulers.Shoot.Strategy != config.MinimalDistance {
			f.Logger.Infof("Skipping Test, because Scheduler is not configured with strategy '%s' but with '%s'", config.MinimalDistance, schedulerConfiguration.Schedulers.Shoot.Strategy)
			return
		}
		// set shoot to a unsupportedRegion where no seed is deployed
		unsupportedRegion, err := ChooseRegionAndZoneWithNoSeed(cloudprofile.Spec.Regions, seeds)
		Expect(err).NotTo(HaveOccurred())
		f.Shoot.Spec.Region = unsupportedRegion.Name
		// setZones(unsupportedRegion)

		// First we create the target shoot.
		shootScheduling := context.WithValue(ctx, nameKey, "schedule shoot, create the shoot and delete the shoot after")

		err = f.GardenerFramework.CreateShoot(shootScheduling, f.Shoot)
		framework.ExpectNoError(err)
		cleanupNeeded = true
		// expecting it to to schedule shoot to seed
		err = f.WaitForShootToBeScheduled(ctx, f.Shoot)
		framework.ExpectNoError(err)

		// We do apiserver scheduling tests here that rely on an existing shoot to avoid having to create another shoot
		By("Api Server ShootBindingStrategy test - wrong scheduling decision: seed does not exist")
		// create invalid Seed
		invalidSeed, err := GenerateInvalidSeed(seeds)
		framework.ExpectNoError(err)

		// retrieve valid shoot
		alreadyScheduledShoot := &gardencorev1beta1.Shoot{}
		err = f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: f.Shoot.Namespace, Name: f.Shoot.Name}, alreadyScheduledShoot)
		framework.ExpectNoError(err)

		err = f.ScheduleShoot(ctx, alreadyScheduledShoot, invalidSeed)
		framework.ExpectNoError(err)

		// double check that invalid seed is not set
		currentShoot := &gardencorev1beta1.Shoot{}
		err = f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: f.Shoot.Namespace, Name: f.Shoot.Name}, currentShoot)
		framework.ExpectNoError(err)
		Expect(*currentShoot.Spec.SeedName).NotTo(Equal(invalidSeed.Name))

		By("Api Server ShootBindingStrategy test - wrong scheduling decision: already scheduled")

		// try to schedule a shoot that is already scheduled to another valid seed
		seed, err := ChooseSeedWhereTestShootIsNotDeployed(currentShoot, seeds)

		if err != nil {
			f.Logger.Warnf("Test not executed: %v", err)
		} else {
			Expect(len(seed.Name)).NotTo(Equal(0))
			err = f.ScheduleShoot(ctx, alreadyScheduledShoot, seed)
			framework.ExpectNoError(err)
		}
	}, WaitForCreateDeleteTimeout)
})

// GenerateInvalidSeed generates a seed with an invalid dummy name
func GenerateInvalidSeed(seeds []gardencorev1beta1.Seed) (*gardencorev1beta1.Seed, error) {
	validSeed := seeds[0]
	if len(validSeed.Name) == 0 {
		return nil, fmt.Errorf("failed to retrieve a valid seed from the current cluster")
	}
	invalidSeed := validSeed.DeepCopy()
	randomString, err := utils.GenerateRandomString(10)
	if err != nil {
		return nil, fmt.Errorf("failed to generate a random string for the name of the seed cluster")
	}
	invalidSeed.ObjectMeta.Name = "dummy-" + randomString
	return invalidSeed, nil
}

// ChooseRegionAndZoneWithNoSeed chooses a region within the cloud provider of the shoot were no seed is deployed and that is allowed by the cloud profile
func ChooseRegionAndZoneWithNoSeed(regions []gardencorev1beta1.Region, seeds []gardencorev1beta1.Seed) (*gardencorev1beta1.Region, error) {
	if len(regions) == 0 {
		return nil, fmt.Errorf("no regions configured in CloudProfile")
	}

	// Find Region where no seed is deployed -
	for _, region := range regions {
		foundRegionInSeed := false
		for _, seed := range seeds {
			if seed.Spec.Provider.Region == region.Name {
				foundRegionInSeed = true
				break
			}
		}
		if !foundRegionInSeed {
			return &region, nil
		}
	}
	return nil, fmt.Errorf("could not determine a region where no seed is deployed already")
}

// ChooseSeedWhereTestShootIsNotDeployed determines a Seed different from the shoot's seed using the same Provider(e.g aws)
// If none can be found, it returns an error
func ChooseSeedWhereTestShootIsNotDeployed(shoot *gardencorev1beta1.Shoot, seeds []gardencorev1beta1.Seed) (*gardencorev1beta1.Seed, error) {
	for _, seed := range seeds {
		if seed.Name != *shoot.Spec.SeedName && seed.Spec.Provider.Type == shoot.Spec.Provider.Type {
			return &seed, nil
		}
	}

	return nil, fmt.Errorf("could not find another seed that is not in use by the test shoot")
}
