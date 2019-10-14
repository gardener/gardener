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
		- Tests the machine image & Kubernetes version maintenance operations for a shoot

	BeforeSuite
		- Prepare valid Shoot from example folder using InfrastructureProvider config
		- If running in TestMachinery mode: scale down the Gardener-Scheduler
		- Create Shoot
		- Update CloudProfile to include a test machine image and a test Kubernetes version

	AfterSuite
		- Delete Shoot and cleanup CloudProfile

	Test: Machine Image Maintenance test
		1) Shoot.Spec.AutoUpdate.MachineImageVersion == false && expirationDate does not apply
		Expected Output
			- shoot machineImage must not be updated in maintenance time
		2) Shoot.Spec.AutoUpdate.MachineImageVersion == true && expirationDate does not apply
		Expected Output
			- shoot machineImage must be updated in maintenance time
		3) Shoot.Spec.AutoUpdate.KubernetesVersion == false && expirationDate does not apply
		Expected Output
			- shoot machineImage must not be updated in maintenance time
		4) Shoot.Spec.AutoUpdate.MachineImageVersion == false && expirationDate applies
		Expected Output
			- shoot machineImage must be updated in maintenance time

	Test: Kubernetes Version Maintenance test
		1) Shoot.Spec.AutoUpdate.KubernetesVersion == false && expirationDate does not apply
		Expected Output
			- shoot Kubernetes version must not be updated in maintenance time
		2) AutoUpdate.KubernetesVersion == false && expirationDate applies
		Expected Output
			- shoot machineImage must be updated in maintenance time
		3) AutoUpdate.KubernetesVersion == true && expirationDate does not apply
		Expected Output
			- shoot machineImage must not be updated in maintenance time
 **/

package maintenance

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/operation/common"
	. "github.com/gardener/gardener/test/integration/shoots"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/test/integration/framework"
)

var (
	kubeconfig                    = flag.String("kubecfg", "", "the path to the kubeconfig of Garden cluster that will be used for integration tests")
	testShootsPrefix              = flag.String("prefix", "", "prefix to use for test shoots")
	logLevel                      = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
	shootMaintenanceTestNamespace = flag.String("shoot-test-namespace", "", "the namespace where the shoot will be created")
	shootMachineImageName         = flag.String("machine-image-name", "", "the Machine Image Name of the test shoot. Defaults to first machine image in the CloudProfile.")
	shootMachineType              = flag.String("machine-type", "", "the Machine type of the first worker of the test shoot. Needs to match the machine types for that Provider available in the CloudProfile")
	testMachineryRun              = flag.Bool("test-machinery-run", false, "indicates whether the test is being executed by the test machinery or locally")
	cloudProfile                  = flag.String("cloud-profile", "", "cloudProfile to use for the shoot")
	shootRegion                   = flag.String("region", "", "region to use for the shoot. Must be compatible with the infrastructureProvider.Zone.")
	secretBinding                 = flag.String("secret-binding", "", "the secretBinding for the provider account of the shoot")
	shootProviderType             = flag.String("provider-type", "", "the type of the cloud provider where the shoot is deployed to. e.g gcp, aws,azure,alicloud")
	shootK8sVersion               = flag.String("k8s-version", "", "kubernetes version to use for the shoot")
	projectNamespace              = flag.String("project-namespace", "", "project namespace where the shoot will be created")
	workerZone                    = flag.String("worker-zone", "", "zone to use for every worker of the shoot.")

	// ProviderConfigs
	infrastructureProviderConfig = flag.String("infrastructure-provider-config-filepath", "", "filepath to the provider specific infrastructure config")

	setupContextTimeout           = time.Minute * 2
	restoreCtxTimeout             = time.Minute * 2
	gardenerSchedulerReplicaCount *int32
	shootMaintenanceTest          *ShootMaintenanceTest

	shootGardenerTest                 *ShootGardenerTest
	intialShootForCreation            gardencorev1alpha1.Shoot
	shootMaintenanceTestLogger        *logrus.Logger
	shootCleanupNeeded                bool
	cloudProfileCleanupNeeded         bool
	testMachineImageVersion           = "0.0.1-beta"
	testKubernetesVersion             = gardencorev1alpha1.ExpirableVersion{Version: "0.0.1"}
	testHighestPatchKubernetesVersion = gardencorev1alpha1.ExpirableVersion{Version: "0.0.5"}
	expirationDateInTheFuture         = metav1.Time{Time: time.Now().Add(time.Second * 20)}
	testMachineImage                  = gardencorev1alpha1.ShootMachineImage{
		Version: testMachineImageVersion,
	}
	shootYamlPath = "/example/90-shoot.yaml"

	trueVar = true
	err     error
)

const (
	WaitForCreateDeleteTimeout = 7200 * time.Second
	InitializationTimeout      = 600 * time.Second
)

func validateFlags() {
	if !StringSet(*kubeconfig) {
		Fail("you need to specify the correct path for the kubeconfig")
	}

	if !FileExists(*kubeconfig) {
		Fail("kubeconfig path does not exist")
	}

	if !StringSet(*logLevel) {
		level := "debug"
		logLevel = &level
	}

	if !StringSet(*shootMaintenanceTestNamespace) {
		Fail("you need to specify the namespace where the shoot will be created")
	}

	if !StringSet(*shootProviderType) {
		Fail("you need to specify provider type of the shoot")
	}

	if !StringSet(*infrastructureProviderConfig) {
		Fail(fmt.Sprintf("you need to specify the filepath to the infrastructureProviderConfig for the provider '%s'", *shootProviderType))
	}

	if !FileExists(*infrastructureProviderConfig) {
		Fail("path to the infrastructureProviderConfig of the Shoot is invalid")
	}
}

var _ = Describe("Shoot Maintenance testing", func() {
	CBeforeSuite(func(ctx context.Context) {
		validateFlags()
		shootMaintenanceTestLogger = logger.AddWriter(logger.NewLogger(*logLevel), GinkgoWriter)

		shootObject := prepareShoot()
		intialShootForCreation = *shootObject.DeepCopy()

		shootGardenerTest, err = NewShootGardenerTest(*kubeconfig, shootObject, shootMaintenanceTestLogger)
		Expect(err).To(BeNil())

		shootGardenerTest.SetupShootWorker(workerZone)
		Expect(err).To(BeNil())
		Expect(len(shootGardenerTest.Shoot.Spec.Provider.Workers)).Should(BeNumerically("==", 1))

		// set machine type & if set, the machineImage name on the first worker image
		if shootMachineType != nil && len(*shootMachineType) > 0 {
			shootGardenerTest.Shoot.Spec.Provider.Workers[0].Machine.Type = *shootMachineType
		}

		if shootMachineImageName != nil && len(*shootMachineImageName) > 0 {
			shootGardenerTest.Shoot.Spec.Provider.Workers[0].Machine.Image.Name = *shootMachineImageName
		}

		shootMaintenanceTest, err = NewShootMaintenanceTest(ctx, shootGardenerTest, shootMachineImageName)
		Expect(err).To(BeNil())
		testMachineImage.Name = shootMaintenanceTest.ShootMachineImage.Name

		if testMachineryRun != nil && *testMachineryRun {
			shootMaintenanceTestLogger.Info("Running in test Machinery")
			zero := int32(0)
			// setup the integration test environment by manipulation the Gardener Components (namespace garden) in the garden cluster
			// scale down the gardener-scheduler to 0 replicas
			replicas, err := ScaleGardenerScheduler(setupContextTimeout, shootMaintenanceTest.ShootGardenerTest.GardenClient.Client(), &zero)
			gardenerSchedulerReplicaCount = replicas
			Expect(err).To(BeNil())
			shootMaintenanceTestLogger.Info("Environment for test-machinery run is prepared")
		}

		// the test machine version is being added to
		prepareCloudProfile(ctx)
		cloudProfileCleanupNeeded = true
	}, InitializationTimeout)

	CAfterSuite(func(ctx context.Context) {
		if cloudProfileCleanupNeeded {
			// retrieve the cloud profile because the machine images & the kubernetes version might got changed during test execution
			err := shootMaintenanceTest.CleanupCloudProfile(ctx, testMachineImage, []gardencorev1alpha1.ExpirableVersion{testKubernetesVersion, testHighestPatchKubernetesVersion})
			Expect(err).NotTo(HaveOccurred())
			shootMaintenanceTestLogger.Infof("Cleaned Cloud Profile '%s'", shootMaintenanceTest.CloudProfile.Name)
		}
		if testMachineryRun != nil && *testMachineryRun {
			_, err := ScaleGardenerScheduler(restoreCtxTimeout, shootMaintenanceTest.ShootGardenerTest.GardenClient.Client(), gardenerSchedulerReplicaCount)
			Expect(err).NotTo(HaveOccurred())
			shootMaintenanceTestLogger.Infof("Environment is restored")
		}
	}, InitializationTimeout)

	CAfterEach(func(ctx context.Context) {
		if shootCleanupNeeded {
			// Finally we delete the shoot again
			shootMaintenanceTestLogger.Infof("Delete shoot %s", shootMaintenanceTest.ShootGardenerTest.Shoot.Name)
			err := shootGardenerTest.DeleteShootAndWaitForDeletion(ctx)
			Expect(err).NotTo(HaveOccurred())
			shootCleanupNeeded = false
		}
	}, WaitForCreateDeleteTimeout)

	CBeforeEach(func(ctx context.Context) {
		if !shootCleanupNeeded {
			// set dummy kubernetes version to shoot
			intialShootForCreation.Spec.Kubernetes.Version = testKubernetesVersion.Version
			// set integration test machineImage to shoot
			intialShootForCreation.Spec.Provider.Workers[0].Machine.Image = &testMachineImage

			shootMaintenanceTest.ShootGardenerTest.Shoot = intialShootForCreation.DeepCopy()
			_, err := shootMaintenanceTest.CreateShoot(ctx)
			Expect(err).NotTo(HaveOccurred())
			shootCleanupNeeded = true
		}
	}, WaitForCreateDeleteTimeout)

	CIt("Machine Image Maintenance test", func(ctx context.Context) {
		By("AutoUpdate.MachineImageVersion == false && expirationDate does not apply -> shoot machineImage must not be updated in maintenance time")
		integrationTestShoot, err := shootGardenerTest.GetShoot(ctx)
		Expect(err).To(BeNil())

		// set test specific shoot settings
		integrationTestShoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = false
		integrationTestShoot.Annotations[common.ShootOperation] = common.ShootOperationMaintain

		// update integration test shoot
		err = shootMaintenanceTest.TryUpdateShootForMachineImageMaintenance(ctx, integrationTestShoot, false, nil)
		Expect(err).To(BeNil())

		err = shootMaintenanceTest.WaitForExpectedMachineImageMaintenance(ctx, testMachineImage, false, time.Now().Add(time.Second*10))
		Expect(err).To(BeNil())

		By("AutoUpdate.MachineImageVersion == true && expirationDate does not apply -> shoot machineImage must be updated in maintenance time")
		// set test specific shoot settings
		integrationTestShoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = trueVar
		integrationTestShoot.Annotations[common.ShootOperation] = common.ShootOperationMaintain

		// update integration test shoot - set maintain now annotation & autoupdate == true
		err = shootMaintenanceTest.TryUpdateShootForMachineImageMaintenance(ctx, integrationTestShoot, false, nil)
		Expect(err).To(BeNil())

		err = shootMaintenanceTest.WaitForExpectedMachineImageMaintenance(ctx, shootMaintenanceTest.ShootMachineImage, true, time.Now().Add(time.Second*20))
		Expect(err).To(BeNil())

		By("AutoUpdate.MachineImageVersion == default && expirationDate does not apply -> shoot machineImage must be updated in maintenance time")
		// set test specific shoot settings
		integrationTestShoot.Spec.Maintenance.AutoUpdate = nil
		integrationTestShoot.Annotations[common.ShootOperation] = common.ShootOperationMaintain

		// reset machine image from latest version to dummy version
		intialShootForCreation.Spec.Provider.Workers[0].Machine.Image = &testMachineImage

		// update integration test shoot - downgrade image again & set maintain now  annotation & autoupdate == nil (default)
		err = shootMaintenanceTest.TryUpdateShootForMachineImageMaintenance(ctx, integrationTestShoot, true, intialShootForCreation.Spec.Provider.Workers)
		Expect(err).To(BeNil())

		err = shootMaintenanceTest.WaitForExpectedMachineImageMaintenance(ctx, shootMaintenanceTest.ShootMachineImage, true, time.Now().Add(time.Second*20))
		Expect(err).To(BeNil())

		By("AutoUpdate.MachineImageVersion == false && expirationDate applies -> shoot machineImage must be updated in maintenance time")
		defer func() {
			// make sure to remove expiration date from cloud profile after test
			err = shootMaintenanceTest.TryUpdateCloudProfileForMaintenance(ctx, shootMaintenanceTest.ShootGardenerTest.Shoot, testMachineImage, nil)
			Expect(err).To(BeNil())
			shootMaintenanceTestLogger.Infof("Cleaned expiration date on machine image from Cloud Profile '%s'", shootMaintenanceTest.CloudProfile.Name)
		}()

		// set test specific shoot settings
		integrationTestShoot.Spec.Maintenance.AutoUpdate = &gardencorev1alpha1.MaintenanceAutoUpdate{MachineImageVersion: false}

		// reset machine image from latest version to dummy version
		intialShootForCreation.Spec.Provider.Workers[0].Machine.Image = &testMachineImage

		// update integration test shoot - downgrade image again & set maintain now  annotation & autoupdate == nil (default)
		err = shootMaintenanceTest.TryUpdateShootForMachineImageMaintenance(ctx, integrationTestShoot, true, intialShootForCreation.Spec.Provider.Workers)
		Expect(err).To(BeNil())

		// modify cloud profile for test
		err = shootMaintenanceTest.TryUpdateCloudProfileForMaintenance(ctx, shootMaintenanceTest.ShootGardenerTest.Shoot, testMachineImage, &expirationDateInTheFuture)
		Expect(err).To(BeNil())

		// sleep so that expiration date is in the past - forceUpdate is required
		time.Sleep(30 * time.Second)
		integrationTestShoot.Annotations[common.ShootOperation] = common.ShootOperationMaintain

		// update integration test shoot - set maintain now  annotation
		err = shootMaintenanceTest.TryUpdateShootForMachineImageMaintenance(ctx, integrationTestShoot, false, nil)
		Expect(err).To(BeNil())

		err = shootMaintenanceTest.WaitForExpectedMachineImageMaintenance(ctx, shootMaintenanceTest.ShootMachineImage, true, time.Now().Add(time.Minute*1))
		Expect(err).To(BeNil())

	}, WaitForCreateDeleteTimeout)

	CIt("Maintenance test - Kubernetes Version force upgrade", func(ctx context.Context) {
		By("AutoUpdate.KubernetesVersion == false && expirationDate does not apply -> shoot Kubernetes version must not be updated in maintenance time")
		integrationTestShoot, err := shootGardenerTest.GetShoot(ctx)
		Expect(err).To(BeNil())

		// set test specific shoot settings
		integrationTestShoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = false
		integrationTestShoot.Annotations[common.ShootOperation] = common.ShootOperationMaintain

		// update integration test shoot
		err = shootMaintenanceTest.TryUpdateShootForKubernetesMaintenance(ctx, integrationTestShoot)
		Expect(err).To(BeNil())

		err = shootMaintenanceTest.WaitForExpectedKubernetesVersionMaintenance(ctx, testKubernetesVersion.Version, false, time.Now().Add(time.Second*10))
		Expect(err).To(BeNil())

		By("AutoUpdate.KubernetesVersion == false && expirationDate applies -> shoot Kubernetes version must be updated in maintenance time")
		defer func() {
			// make sure to remove expiration date from cloud profile after test
			err = shootMaintenanceTest.TryUpdateCloudProfileForKubernetesVersionMaintenance(ctx, shootMaintenanceTest.ShootGardenerTest.Shoot, testKubernetesVersion.Version, nil)
			Expect(err).To(BeNil())
			shootMaintenanceTestLogger.Infof("Cleaned expiration date on kubernetes version from Cloud Profile '%s'", shootMaintenanceTest.CloudProfile.Name)
		}()

		// modify cloud profile for test
		err = shootMaintenanceTest.TryUpdateCloudProfileForKubernetesVersionMaintenance(ctx, shootMaintenanceTest.ShootGardenerTest.Shoot, testKubernetesVersion.Version, &expirationDateInTheFuture)
		Expect(err).To(BeNil())

		// set test specific shoot settings
		integrationTestShoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = false

		// update integration test shoot - autoupdate == false
		err = shootMaintenanceTest.TryUpdateShootForKubernetesMaintenance(ctx, integrationTestShoot)
		Expect(err).To(BeNil())

		// sleep so that expiration date is in the past - forceUpdate is required
		time.Sleep(30 * time.Second)
		integrationTestShoot.Annotations[common.ShootOperation] = common.ShootOperationMaintain

		// update integration test shoot - set maintain now  annotation
		err = shootMaintenanceTest.TryUpdateShootForKubernetesMaintenance(ctx, integrationTestShoot)
		Expect(err).To(BeNil())

		err = shootMaintenanceTest.WaitForExpectedKubernetesVersionMaintenance(ctx, testHighestPatchKubernetesVersion.Version, true, time.Now().Add(time.Second*20))
		Expect(err).To(BeNil())

	}, WaitForCreateDeleteTimeout)

	CIt("Maintenance test - Kubernetes Version auto upgrade", func(ctx context.Context) {
		By("AutoUpdate.KubernetesVersion == true && expirationDate does not apply -> shoot Kubernetes version must not be updated in maintenance time")
		integrationTestShoot, err := shootGardenerTest.GetShoot(ctx)
		Expect(err).To(BeNil())

		// set test specific shoot settings
		integrationTestShoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = trueVar
		integrationTestShoot.Annotations[common.ShootOperation] = common.ShootOperationMaintain

		// update integration test shoot
		err = shootMaintenanceTest.TryUpdateShootForKubernetesMaintenance(ctx, integrationTestShoot)
		Expect(err).To(BeNil())

		err = shootMaintenanceTest.WaitForExpectedKubernetesVersionMaintenance(ctx, testHighestPatchKubernetesVersion.Version, true, time.Now().Add(time.Second*20))
		Expect(err).To(BeNil())
	}, WaitForCreateDeleteTimeout)
})

func prepareCloudProfile(ctx context.Context) {
	// setup cloud profile for integration test
	found, image, err := helper.DetermineMachineImageForName(shootMaintenanceTest.CloudProfile, shootMaintenanceTest.ShootMachineImage.Name)
	Expect(err).To(BeNil())
	Expect(found).To(Equal(true))
	imageVersions := append(image.Versions, gardencorev1alpha1.ExpirableVersion{Version: testMachineImageVersion})
	updatedCloudProfileImages, err := helper.SetMachineImageVersionsToMachineImage(shootMaintenanceTest.CloudProfile.Spec.MachineImages, shootMaintenanceTest.ShootMachineImage.Name, imageVersions)
	Expect(err).To(BeNil())
	// need one image in Cloud Profile to be updated with one additional version
	shootMaintenanceTest.CloudProfile.Spec.MachineImages = updatedCloudProfileImages
	// add my test kubernetes versions (one low patch version, one high patch version)
	shootMaintenanceTest.CloudProfile.Spec.Kubernetes.Versions = append(shootMaintenanceTest.CloudProfile.Spec.Kubernetes.Versions, testKubernetesVersion, testHighestPatchKubernetesVersion)
	// update Cloud Profile with integration test machineImage & kubernetes version
	err = shootGardenerTest.GardenClient.Client().Update(ctx, shootMaintenanceTest.CloudProfile)
	Expect(err).To(BeNil())
	return
}

func prepareShoot() *gardencorev1alpha1.Shoot {
	// if running in test machinery, test will be executed from root of the project
	if !FileExists(fmt.Sprintf(".%s", shootYamlPath)) {
		// locally, we need find the example shoot
		shootYamlPath = GetProjectRootPath() + shootYamlPath
		Expect(FileExists(shootYamlPath)).To(Equal(true))
	}
	// parse shoot yaml into shoot object and generate random test names for shoots
	_, shootObject, err := CreateShootTestArtifacts(shootYamlPath, testShootsPrefix, projectNamespace, shootRegion, cloudProfile, secretBinding, shootProviderType, shootK8sVersion, nil, true, true)
	Expect(err).To(BeNil())

	// parse Infrastructure config
	infrastructureProviderConfig, err := ParseFileAsProviderConfig(*infrastructureProviderConfig)
	Expect(err).To(BeNil())
	shootObject.Spec.Provider.InfrastructureConfig = infrastructureProviderConfig
	// set other provider configs to nil as we do not need them for shoot creation
	shootObject.Spec.Provider.ControlPlaneConfig = nil
	return shootObject
}
