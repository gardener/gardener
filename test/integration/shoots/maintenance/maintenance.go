// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
		- Tests the machine image & Kubernetes version maintenance operations for a shoot

	BeforeSuite
		- Prepare valid Shoot from example folder using InfrastructureProvider config
		- If running in TestMachinery mode: scale down the Gardener-Scheduler
		- Update CloudProfile to include a test machine image and tests Kubernetes versions

	AfterSuite
		- Delete Shoot and cleanup CloudProfile

	Test: Machine Image Maintenance test
		1) Shoot.Spec.AutoUpdate.MachineImageVersion == false && expirationDate does not apply
		Expected Output
			- shoot machineImage must not be updated in maintenance time
		2) Shoot.Spec.AutoUpdate.MachineImageVersion == true && expirationDate does not apply
		Expected Output
			- shoot machineImage must be updated in maintenance time

		3) Shoot.Spec.AutoUpdate.MachineImageVersion == false && expirationDate applies
		Expected Output
			- shoot machineImage must be updated in maintenance time

	Test: Kubernetes Version Maintenance test
		1) Shoot.Spec.AutoUpdate.KubernetesVersion == false && expirationDate does not apply
		Expected Output
			- shoot Kubernetes version must not be updated in maintenance time
		2) AutoUpdate.KubernetesVersion == true && expirationDate does not apply
		Expected Output
			- shoot Kubernetes version must not be updated in maintenance time
		3) Patch Version update: AutoUpdate.KubernetesVersion == false && expirationDate applies
		Expected Output
			- shoot Kubernetes version must be updated in maintenance time to highest patch version of its minor version
		4) Minor Version update: AutoUpdate.KubernetesVersion == false && expirationDate applies
		Expected Output
			- shoot Kubernetes version must be updated in maintenance time to highest patch version of next minor version
 **/

package maintenance

import (
	"context"
	"flag"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/test/framework"
)

// TODO: this test is currently not executed, because it has to manipulate the CloudProfile. It should be refactor into
//  an envtest-style integration test, so that we can enable it again.

var (
	testMachineryRun      = flag.Bool("test-machinery-run", false, "indicates whether the test is being executed by the test machinery or locally")
	testShootCloudProfile gardencorev1beta1.CloudProfile

	// Test Machine Image
	highestShootMachineImage gardencorev1beta1.ShootMachineImage
	testMachineImageVersion  = "0.0.1-beta"
	testMachineImage         = gardencorev1beta1.ShootMachineImage{
		Version: &testMachineImageVersion,
	}

	// Test Kubernetes versions
	testKubernetesVersionLowPatchLowMinor             = gardencorev1beta1.ExpirableVersion{Version: "0.0.1", Classification: &deprecatedClassification}
	testKubernetesVersionHighestPatchLowMinor         = gardencorev1beta1.ExpirableVersion{Version: "0.0.5", Classification: &deprecatedClassification}
	testKubernetesVersionLowPatchConsecutiveMinor     = gardencorev1beta1.ExpirableVersion{Version: "0.1.1", Classification: &deprecatedClassification}
	testKubernetesVersionHighestPatchConsecutiveMinor = gardencorev1beta1.ExpirableVersion{Version: "0.1.5", Classification: &deprecatedClassification}

	// cleanup
	gardenerSchedulerReplicaCount *int32
	baseShoot                     gardencorev1beta1.Shoot
	shootCleanupNeeded            bool
	cloudProfileCleanupNeeded     bool

	// other
	deprecatedClassification = gardencorev1beta1.ClassificationDeprecated
	expirationDateInThePast  = metav1.Time{Time: time.Now().UTC().AddDate(0, 0, -1)}
	err                      error
	f                        *framework.ShootCreationFramework
)

func init() {
	framework.RegisterShootCreationFrameworkFlags()
}

const (
	waitForCreateDeleteTimeout = 7200 * time.Second
	initializationTimeout      = 600 * time.Second
)

var _ = ginkgo.Describe("Shoot Maintenance testing", func() {
	f = framework.NewShootCreationFramework(&framework.ShootCreationConfig{
		GardenerConfig: &framework.GardenerConfig{
			CommonConfig: &framework.CommonConfig{
				ResourceDir: "../../../framework/resources",
			},
		},
	})

	framework.CBeforeSuite(func(ctx context.Context) {
		if testMachineryRun != nil && *testMachineryRun {
			f.Logger.Info("Running in test Machinery")
			// setup the integration test environment by manipulation the Gardener Components (namespace garden) in the garden cluster
			// scale down the gardener-scheduler to 0 replicas
			replicas, err := framework.ScaleGardenerScheduler(ctx, f.GardenClient.Client(), pointer.Int32(0))
			gardenerSchedulerReplicaCount = replicas
			gomega.Expect(err).To(gomega.BeNil())
			f.Logger.Info("Environment for test-machinery run is prepared")
		}

		f.CommonFramework.BeforeEach()
		// this is needed to initialize the Gardener framework in the BeforeSuite()
		// otherwise the initialize happens in the beforeEach() - so there is no garden client available
		f.GardenerFramework.BeforeEach()
		// required to initialize the Shoot configuration via the provided flags
		f.BeforeEach()

		// validateFlags()
		err = f.InitializeShootWithFlags(ctx)
		gomega.Expect(err).To(gomega.BeNil())
		gomega.Expect(len(f.Shoot.Spec.Provider.Workers)).Should(gomega.BeNumerically("==", 1))
		gomega.Expect(f.Shoot.Spec.Provider.Workers[0].Machine.Image).Should(gomega.Not(gomega.BeNil()))

		// turn off auto update on Kubernetes and Machine image versions
		f.Shoot.Spec.Maintenance = &gardencorev1beta1.Maintenance{
			AutoUpdate: &gardencorev1beta1.MaintenanceAutoUpdate{
				KubernetesVersion:   false,
				MachineImageVersion: false,
			},
		}
		// remember highest version of the image.
		highestShootMachineImage = *f.Shoot.Spec.Provider.Workers[0].Machine.Image
		// set dummy kubernetes version to shoot
		f.Shoot.Spec.Kubernetes.Version = testKubernetesVersionLowPatchLowMinor.Version
		// set test version
		f.Shoot.Spec.Provider.Workers[0].Machine.Image.Version = testMachineImage.Version
		// remember the test machine image
		// also required to know for which image name the test versions should be added to the CloudProfile
		testMachineImage = *f.Shoot.Spec.Provider.Workers[0].Machine.Image
		// base Shoot is once set in the beginning and is used as a configuration in case the Shoot has been altered
		baseShoot = *f.Shoot.DeepCopy()

		profile, err := f.GetCloudProfile(ctx, f.Shoot.Spec.CloudProfileName)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		testShootCloudProfile = *profile

		// prepare the CloudProfile with the test machine image version and the test kubernetes versions
		prepareCloudProfile(ctx, testShootCloudProfile, f.Shoot)
		cloudProfileCleanupNeeded = true
	}, initializationTimeout)

	framework.CAfterSuite(func(ctx context.Context) {
		framework.CommonAfterSuite()
		if cloudProfileCleanupNeeded {
			err := CleanupCloudProfile(ctx, f.GardenClient.Client(), testShootCloudProfile.Name, testMachineImage, []gardencorev1beta1.ExpirableVersion{testKubernetesVersionLowPatchLowMinor, testKubernetesVersionHighestPatchLowMinor, testKubernetesVersionLowPatchConsecutiveMinor, testKubernetesVersionHighestPatchConsecutiveMinor})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			f.Logger.Infof("Cleaned Cloud Profile '%s'", testShootCloudProfile.Name)
		}
		if testMachineryRun != nil && *testMachineryRun {
			_, err := framework.ScaleGardenerScheduler(ctx, f.GardenClient.Client(), gardenerSchedulerReplicaCount)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			f.Logger.Infof("Environment is restored")
		}
	}, initializationTimeout)

	framework.CBeforeEach(func(ctx context.Context) {
		if !shootCleanupNeeded {
			err = f.GetShoot(ctx, f.Shoot)
			gomega.Expect(apierrors.IsNotFound(err)).To(gomega.Equal(true))

			// create the shoot based on the base Shoot with test configuration
			f.Shoot = baseShoot.DeepCopy()
			_, err := f.CreateShoot(ctx, false, false)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			shootCleanupNeeded = true
		}
	}, waitForCreateDeleteTimeout)

	framework.CAfterEach(func(ctx context.Context) {
		if shootCleanupNeeded {
			// Finally we delete the shoot again
			f.Logger.Infof("Delete shoot %s", f.Shoot.Name)
			err := f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			shootCleanupNeeded = false
		}
	}, waitForCreateDeleteTimeout)

	ginkgo.Describe("Machine image maintenance tests", func() {
		f.Beta().Serial().CIt("Do not update Shoot machine image in maintenance time: AutoUpdate.MachineImageVersion == false && expirationDate does not apply", func(ctx context.Context) {
			err = StartShootMaintenance(ctx, f.GardenClient.Client(), f.Shoot)
			gomega.Expect(err).To(gomega.BeNil())
			err = WaitForExpectedMachineImageMaintenance(ctx, f.Logger, f.GardenClient.Client(), f.Shoot, testMachineImage, false, time.Now().Add(time.Second*10))
			gomega.Expect(err).To(gomega.BeNil())
		}, waitForCreateDeleteTimeout)

		f.Beta().Serial().CIt("Shoot machine image must be updated in maintenance time: AutoUpdate.MachineImageVersion == true && expirationDate does not apply", func(ctx context.Context) {
			// set test specific shoot settings
			patch := client.MergeFrom(f.Shoot.DeepCopy())
			f.Shoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = true
			framework.ExpectNoError(f.GardenClient.Client().Patch(ctx, f.Shoot, patch))

			err = StartShootMaintenance(ctx, f.GardenClient.Client(), f.Shoot)
			gomega.Expect(err).To(gomega.BeNil())

			err = WaitForExpectedMachineImageMaintenance(ctx, f.Logger, f.GardenClient.Client(), f.Shoot, highestShootMachineImage, true, time.Now().Add(time.Second*20))
			gomega.Expect(err).To(gomega.BeNil())
		}, waitForCreateDeleteTimeout)

		f.Beta().Serial().CIt("Shoot machine image must be updated in maintenance time: AutoUpdate.MachineImageVersion == false && expirationDate applies", func(ctx context.Context) {
			defer func() {
				// make sure to remove expiration date from cloud profile after test
				err = PatchCloudProfileForMachineImageMaintenance(ctx, f.GardenClient.Client(), f.Shoot.Spec.CloudProfileName, testMachineImage, nil, &deprecatedClassification)
				gomega.Expect(err).To(gomega.BeNil())
				f.Logger.Infof("Cleaned expiration date on machine image from Cloud Profile '%s'", testShootCloudProfile.Name)
			}()
			// expire the Shoot's machine image
			err = PatchCloudProfileForMachineImageMaintenance(ctx, f.GardenClient.Client(), f.Shoot.Spec.CloudProfileName, testMachineImage, &expirationDateInThePast, &deprecatedClassification)
			gomega.Expect(err).To(gomega.BeNil())

			// give controller caches time to sync
			time.Sleep(10 * time.Second)

			err = StartShootMaintenance(ctx, f.GardenClient.Client(), f.Shoot)
			gomega.Expect(err).To(gomega.BeNil())

			err = WaitForExpectedMachineImageMaintenance(ctx, f.Logger, f.GardenClient.Client(), f.Shoot, highestShootMachineImage, true, time.Now().Add(time.Minute*1))
			gomega.Expect(err).To(gomega.BeNil())
		}, waitForCreateDeleteTimeout)
	})

	ginkgo.Describe("Kubernetes version maintenance tests", func() {
		f.Beta().Serial().CIt("Kubernetes version should not be updated: auto update not enabled", func(ctx context.Context) {
			err = StartShootMaintenance(ctx, f.GardenClient.Client(), f.Shoot)
			gomega.Expect(err).To(gomega.BeNil())

			err = WaitForExpectedKubernetesVersionMaintenance(ctx, f.Logger, f.GardenClient.Client(), f.Shoot, testKubernetesVersionLowPatchLowMinor.Version, false, time.Now().Add(time.Second*10))
			gomega.Expect(err).To(gomega.BeNil())

		}, waitForCreateDeleteTimeout)

		f.Beta().Serial().CIt("Kubernetes version should be updated: auto update enabled", func(ctx context.Context) {
			// set test specific shoot settings
			patch := client.MergeFrom(f.Shoot.DeepCopy())
			f.Shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = true
			framework.ExpectNoError(f.GardenClient.Client().Patch(ctx, f.Shoot, patch))

			err = StartShootMaintenance(ctx, f.GardenClient.Client(), f.Shoot)
			gomega.Expect(err).To(gomega.BeNil())

			err = WaitForExpectedKubernetesVersionMaintenance(ctx, f.Logger, f.GardenClient.Client(), f.Shoot, testKubernetesVersionHighestPatchLowMinor.Version, true, time.Now().Add(time.Second*20))
			gomega.Expect(err).To(gomega.BeNil())
		}, waitForCreateDeleteTimeout)

		f.Beta().Serial().CIt("Kubernetes version should be updated: force update patch version", func(ctx context.Context) {
			defer func() {
				// make sure to remove expiration date from cloud profile after test
				err = PatchCloudProfileForKubernetesVersionMaintenance(ctx, f.GardenClient.Client(), f.Shoot.Spec.CloudProfileName, testKubernetesVersionLowPatchLowMinor.Version, nil, &deprecatedClassification)
				gomega.Expect(err).To(gomega.BeNil())
				f.Logger.Infof("Cleaned expiration date on kubernetes version from Cloud Profile '%s'", testShootCloudProfile.Name)
			}()

			// expire the Shoot's Kubernetes version
			err = PatchCloudProfileForKubernetesVersionMaintenance(ctx, f.GardenClient.Client(), f.Shoot.Spec.CloudProfileName, testKubernetesVersionLowPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)
			gomega.Expect(err).To(gomega.BeNil())

			// give controller caches time to sync
			time.Sleep(10 * time.Second)

			err = StartShootMaintenance(ctx, f.GardenClient.Client(), f.Shoot)
			gomega.Expect(err).To(gomega.BeNil())

			err = WaitForExpectedKubernetesVersionMaintenance(ctx, f.Logger, f.GardenClient.Client(), f.Shoot, testKubernetesVersionHighestPatchLowMinor.Version, true, time.Now().Add(time.Second*20))
			gomega.Expect(err).To(gomega.BeNil())
		}, waitForCreateDeleteTimeout)

		f.Beta().Serial().CIt("Kubernetes version should be updated: force update minor version", func(ctx context.Context) {
			defer func() {
				// make sure to remove expiration date from cloud profile after test
				err = PatchCloudProfileForKubernetesVersionMaintenance(ctx, f.GardenClient.Client(), f.Shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)
				gomega.Expect(err).To(gomega.BeNil())
				f.Logger.Infof("Cleaned expiration date on kubernetes version from Cloud Profile '%s'", testShootCloudProfile.Name)
			}()

			// set the shoots Kubernetes version to be the highest patch version of the minor version
			patch := client.MergeFrom(f.Shoot.DeepCopy())
			f.Shoot.Spec.Kubernetes.Version = testKubernetesVersionHighestPatchLowMinor.Version
			framework.ExpectNoError(f.GardenClient.Client().Patch(ctx, f.Shoot, patch))

			// let Shoot's Kubernetes version expire
			err = PatchCloudProfileForKubernetesVersionMaintenance(ctx, f.GardenClient.Client(), f.Shoot.Spec.CloudProfileName, testKubernetesVersionHighestPatchLowMinor.Version, &expirationDateInThePast, &deprecatedClassification)
			gomega.Expect(err).To(gomega.BeNil())

			// give controller caches time to sync
			time.Sleep(10 * time.Second)

			err = StartShootMaintenance(ctx, f.GardenClient.Client(), f.Shoot)
			gomega.Expect(err).To(gomega.BeNil())

			// expect shoot to have updated to latest patch version of next minor version
			err = WaitForExpectedKubernetesVersionMaintenance(ctx, f.Logger, f.GardenClient.Client(), f.Shoot, testKubernetesVersionHighestPatchConsecutiveMinor.Version, true, time.Now().Add(time.Second*20))
			gomega.Expect(err).To(gomega.BeNil())
		}, waitForCreateDeleteTimeout)
	})
})

func prepareCloudProfile(ctx context.Context, profile gardencorev1beta1.CloudProfile, shoot *gardencorev1beta1.Shoot) {
	// setup cloud profile for integration test
	machineImageName := shoot.Spec.Provider.Workers[0].Machine.Image.Name
	found, image, err := helper.DetermineMachineImageForName(&profile, machineImageName)
	gomega.Expect(err).To(gomega.BeNil())
	gomega.Expect(found).To(gomega.Equal(true))

	imageVersions := append(
		image.Versions,
		gardencorev1beta1.MachineImageVersion{
			ExpirableVersion: gardencorev1beta1.ExpirableVersion{
				Version:        testMachineImageVersion,
				Classification: &deprecatedClassification,
			},
		})
	updatedCloudProfileImages, err := helper.SetMachineImageVersionsToMachineImage(profile.Spec.MachineImages, machineImageName, imageVersions)
	gomega.Expect(err).To(gomega.BeNil())
	// need one image in Cloud Profile to be updated with one additional version
	profile.Spec.MachineImages = updatedCloudProfileImages

	// add  test kubernetes versions (one low patch version, one high patch version)
	profile.Spec.Kubernetes.Versions = append(profile.Spec.Kubernetes.Versions, testKubernetesVersionLowPatchLowMinor, testKubernetesVersionHighestPatchLowMinor, testKubernetesVersionLowPatchConsecutiveMinor, testKubernetesVersionHighestPatchConsecutiveMinor)
	err = f.GardenClient.Client().Update(ctx, &profile)
	gomega.Expect(err).To(gomega.BeNil())
}
