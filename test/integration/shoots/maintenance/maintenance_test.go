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

package maintenance

import (
	"context"
	"flag"
	"time"

	"github.com/gardener/gardener/pkg/operation/common"
	. "github.com/gardener/gardener/test/integration/shoots"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/test/integration/framework"
)

var (
	kubeconfig                    = flag.String("kubeconfig", "", "the path to the kubeconfig of Garden cluster that will be used for integration tests")
	shootTestYamlPath             = flag.String("shootpath", "", "the path to the shoot yaml that will be used for testing")
	testShootsPrefix              = flag.String("prefix", "", "prefix to use for test shoots")
	logLevel                      = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
	shootMaintenanceTestNamespace = flag.String("shoot-test-namespace", "", "the namespace where the shoot will be created")
)

const (
	WaitForCreateDeleteTimeout = 7200 * time.Second
	InitializationTimeout      = 600 * time.Second
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

	if !StringSet(*shootMaintenanceTestNamespace) {
		Fail("you need to specify the namespace where the shoot will be created")
	}
}

var _ = Describe("Shoot Maintenance testing", func() {
	var (
		shootGardenerTest          *ShootGardenerTest
		shootMaintenanceTest       *ShootMaintenanceTest
		shoot                      *v1beta1.Shoot
		shootMaintenanceTestLogger *logrus.Logger
		shootCleanupNeeded         bool
		cloudProfileCleanupNeeded  bool
		testMachineImageVersion    = "0.0.1-beta"

		testMachineImage = gardenv1beta1.ShootMachineImage{
			Version: testMachineImageVersion,
		}
		falseVar = false
		trueVar  = true
	)

	CBeforeSuite(func(ctx context.Context) {
		validateFlags()

		// parse shoot yaml into shoot object and generate random test names for shoots
		_, shootObject, err := CreateShootTestArtifacts(*shootTestYamlPath, *testShootsPrefix, falseVar)
		Expect(err).To(BeNil())

		shoot = shootObject
		shootMaintenanceTestLogger = logger.AddWriter(logger.NewLogger(*logLevel), GinkgoWriter)

		shootGardenerTest, err = NewShootGardenerTest(*kubeconfig, shoot, shootMaintenanceTestLogger)
		Expect(err).To(BeNil())

		shootMaintenanceTest, err = NewShootMaintenanceTest(ctx, shootGardenerTest)
		Expect(err).To(BeNil())

		// the test machine version is being added to
		testMachineImage.Name = shootMaintenanceTest.ShootMachineImage.Name

		// setup cloud profile & shoot for integration test
		found, image, err := helper.DetermineMachineImageForName(*shootMaintenanceTest.CloudProfile, shootMaintenanceTest.ShootMachineImage.Name)
		Expect(err).To(BeNil())
		Expect(found).To(Equal(true))

		cloudProfileImages, err := helper.GetMachineImagesFromCloudProfile(shootMaintenanceTest.CloudProfile)
		Expect(err).To(BeNil())
		Expect(cloudProfileImages).NotTo(BeNil())

		imageVersions := append(image.Versions, gardenv1beta1.MachineImageVersion{Version: testMachineImageVersion})
		updatedCloudProfileImages, err := helper.SetMachineImageVersionsToMachineImage(cloudProfileImages, shootMaintenanceTest.ShootMachineImage.Name, imageVersions)
		Expect(err).To(BeNil())

		// need one image in Cloud Profile to be updated with one additional version
		err = helper.SetMachineImages(shootMaintenanceTest.CloudProfile, updatedCloudProfileImages)
		Expect(err).To(BeNil())

		// update Cloud Profile with integration test machineImage
		cloudProfile, err := shootGardenerTest.GardenClient.Garden().GardenV1beta1().CloudProfiles().Update(shootMaintenanceTest.CloudProfile)
		Expect(err).To(BeNil())
		Expect(cloudProfile).NotTo(BeNil())
		cloudProfileCleanupNeeded = true

		// set integration test machineImage to shoot
		updateImage := helper.UpdateMachineImage(shootMaintenanceTest.CloudProvider, &testMachineImage)
		Expect(updateImage).NotTo(BeNil())
		updateImage(&shoot.Spec.Cloud)

		_, err = shootMaintenanceTest.CreateShoot(ctx)
		Expect(err).NotTo(HaveOccurred())
		shootCleanupNeeded = true
	}, InitializationTimeout)

	CAfterSuite(func(ctx context.Context) {
		if cloudProfileCleanupNeeded {
			// retrieve the cloud profile because the machine images might got changed during test execution
			err := shootMaintenanceTest.RemoveTestMachineImageVersionFromCloudProfile(ctx, testMachineImage)
			Expect(err).NotTo(HaveOccurred())
			shootMaintenanceTestLogger.Infof("Cleaned Cloud Profile '%s'", shootMaintenanceTest.CloudProfile.Name)
		}

		if shootCleanupNeeded {
			// Finally we delete the shoot again
			shootMaintenanceTestLogger.Infof("Delete shoot %s", shoot.Name)
			err := shootGardenerTest.DeleteShoot(ctx)
			Expect(err).NotTo(HaveOccurred())
		}
	}, InitializationTimeout)

	CIt("Maintenance test with shoot ", func(ctx context.Context) {
		By("AutoUpdate.MachineImageVersion == false && expirationDate does not apply -> shoot machineImage must not be updated in maintenance time")
		integrationTestShoot, err := shootGardenerTest.GetShoot(ctx)
		Expect(err).To(BeNil())

		// set test specific shoot settings
		integrationTestShoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = &falseVar
		integrationTestShoot.Annotations[common.ShootOperation] = common.ShootOperationMaintain

		// update integration test shoot
		err = shootMaintenanceTest.TryUpdateShootForMaintenance(ctx, integrationTestShoot, false, nil)
		Expect(err).To(BeNil())

		err = shootMaintenanceTest.WaitForExpectedMaintenance(ctx, testMachineImage, shootMaintenanceTest.CloudProvider, false, time.Now().Add(time.Minute*1))
		Expect(err).To(BeNil())

		By("AutoUpdate.MachineImageVersion == true && expirationDate does not apply -> shoot machineImage must be updated in maintenance time")
		// set test specific shoot settings
		integrationTestShoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = &trueVar
		integrationTestShoot.Annotations[common.ShootOperation] = common.ShootOperationMaintain

		// update integration test shoot - set maintain now annotation & autoupdate == true
		err = shootMaintenanceTest.TryUpdateShootForMaintenance(ctx, integrationTestShoot, false, nil)
		Expect(err).To(BeNil())

		err = shootMaintenanceTest.WaitForExpectedMaintenance(ctx, shootMaintenanceTest.ShootMachineImage, shootMaintenanceTest.CloudProvider, true, time.Now().Add(time.Minute*1))
		Expect(err).To(BeNil())

		By("AutoUpdate.MachineImageVersion == default && expirationDate does not apply -> shoot machineImage must be updated in maintenance time")
		// set test specific shoot settings
		integrationTestShoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = nil
		integrationTestShoot.Annotations[common.ShootOperation] = common.ShootOperationMaintain

		// reset machine image from latest version to dummy version
		updateImage := helper.UpdateMachineImage(shootMaintenanceTest.CloudProvider, &testMachineImage)
		Expect(updateImage).NotTo(BeNil())

		// update integration test shoot - downgrade image again & set maintain now  annotation & autoupdate == nil (default)
		err = shootMaintenanceTest.TryUpdateShootForMaintenance(ctx, integrationTestShoot, true, updateImage)
		Expect(err).To(BeNil())

		err = shootMaintenanceTest.WaitForExpectedMaintenance(ctx, shootMaintenanceTest.ShootMachineImage, shootMaintenanceTest.CloudProvider, true, time.Now().Add(time.Minute*1))
		Expect(err).To(BeNil())

		By("AutoUpdate.MachineImageVersion == false && expirationDate does apply -> shoot machineImage must be updated in maintenance time")
		// modify cloud profile for test
		err = shootMaintenanceTest.TryUpdateCloudProfileForMaintenance(ctx, shoot, testMachineImage)
		Expect(err).To(BeNil())

		// set test specific shoot settings
		integrationTestShoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = &falseVar

		// reset machine image from latest version to dummy version
		updateImage = helper.UpdateMachineImage(shootMaintenanceTest.CloudProvider, &testMachineImage)
		Expect(updateImage).NotTo(BeNil())

		// update integration test shoot - downgrade image again & set maintain now  annotation & autoupdate == nil (default)
		err = shootMaintenanceTest.TryUpdateShootForMaintenance(ctx, integrationTestShoot, true, updateImage)
		Expect(err).To(BeNil())

		//sleep so that expiration date is in the past - forceUpdate is required
		time.Sleep(30 * time.Second)
		integrationTestShoot.Annotations[common.ShootOperation] = common.ShootOperationMaintain

		// update integration test shoot - set maintain now  annotation
		err = shootMaintenanceTest.TryUpdateShootForMaintenance(ctx, integrationTestShoot, true, updateImage)
		Expect(err).To(BeNil())

		err = shootMaintenanceTest.WaitForExpectedMaintenance(ctx, shootMaintenanceTest.ShootMachineImage, shootMaintenanceTest.CloudProvider, true, time.Now().Add(time.Minute*1))
		Expect(err).To(BeNil())

	}, WaitForCreateDeleteTimeout)
})
