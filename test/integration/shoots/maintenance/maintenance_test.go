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
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	. "github.com/gardener/gardener/test/integration/shoots"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/logger"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/test/integration/framework"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

var (
	kubeconfig                    = flag.String("kubeconfig", "", "the path to the kubeconfig of Garden cluster that will be used for integration tests")
	shootTestYamlPath             = flag.String("shootpath", "", "the path to the shoot yaml that will be used for testing")
	testShootsPrefix              = flag.String("prefix", "", "prefix to use for test shoots")
	logLevel                      = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
	shootMaintenanceTestNamespace = flag.String("shoot-test-namespace", "", "the namespace where the shoot will be created")
	testMachineryCloudProvider    = flag.String("cloudprovider", "", "the CloudProvider of the integration test shoot")
	shootMachineImageName         = flag.String("machine-image-name", "", "the Machine Image Name of the test shoot. Needs to be set when not specifying a shootpath")
	testMachineryRun              = flag.Bool("test-machinery-run", false, "indicates whether the test is being executed by the test machinery or locally")
	// timeouts
	setupContextTimeout = time.Minute * 2
	restoreCtxTimeout   = time.Minute * 2

	gardenerSchedulerReplicaCount *int32

	shootMaintenanceTest *ShootMaintenanceTest
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
	} else {
		// path := fmt.Sprintf("example/90-shoot-%s.yaml", *testMachineryCloudProvider)
		path := fmt.Sprintf("/Users/d060239/go/src/github.com/gardener/gardener/example/90-shoot-%s.yaml", *testMachineryCloudProvider)
		shootTestYamlPath = &path

		if !StringSet(*shootMachineImageName) {
			Fail("you need to specify the Machine Image name of the test shoot")
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

	if testMachineryRun != nil && *testMachineryRun && !StringSet(*testMachineryCloudProvider) {
		Fail("you need to specify the CloudProvider when running in test-machinery mode")
	}
}

// setup the integration test environment by manipulation the Gardener Components (namespace garden) in the garden cluster
// scale down the gardener-scheduler to 0 replicas
func setupEnvironmentForMaintenanceTest() error {
	ctxSetup, cancelCtxSetup := context.WithTimeout(context.Background(), setupContextTimeout)
	defer cancelCtxSetup()

	replicas, err := GetDeploymentReplicas(ctxSetup, shootMaintenanceTest.ShootGardenerTest.GardenClient.Client(), "garden", "gardener-scheduler")
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to retrieve the replica count of the Gardener-scheduler deployment: '%v'", err)
	}
	if replicas == nil || *replicas == 0 {
		return nil
	}
	gardenerSchedulerReplicaCount = replicas

	// scale down the scheduler deployment
	if err := kubernetes.ScaleDeployment(ctxSetup, shootMaintenanceTest.ShootGardenerTest.GardenClient.Client(), kutil.Key("garden", "gardener-scheduler"), 0); err != nil {
		return fmt.Errorf("failed to scale down the replica count of the Gardener-scheduler deployment: '%v'", err)
	}

	// wait until scaled down
	if err := WaitUntilDeploymentScaled(ctxSetup, shootMaintenanceTest.ShootGardenerTest.GardenClient.Client(), "garden", "gardener-scheduler", 0); err != nil {
		return fmt.Errorf("failed to wait until the Gardener-scheduler deployment is scaled down: '%v'", err)
	}
	return nil
}

// restoreEnvironment restores the Gardener components like they were before the maintenance test
func restoreEnvironment() error {
	if gardenerSchedulerReplicaCount == nil {
		return nil
	}

	ctxRestore, cancelCtxRestore := context.WithTimeout(context.Background(), restoreCtxTimeout)

	defer cancelCtxRestore()

	// scale up the scheduler deployment
	if err := kubernetes.ScaleDeployment(ctxRestore, shootMaintenanceTest.ShootGardenerTest.GardenClient.Client(), kutil.Key("garden", "gardener-scheduler"), *gardenerSchedulerReplicaCount); err != nil {
		return fmt.Errorf("failed to restore the environment after the integration test. Scaling up the replica count of the Gardener-scheduler deployment failed: '%v'", err)
	}

	// wait until scaled up
	if err := WaitUntilDeploymentScaled(ctxRestore, shootMaintenanceTest.ShootGardenerTest.GardenClient.Client(), "garden", "gardener-scheduler", *gardenerSchedulerReplicaCount); err != nil {
		return fmt.Errorf("failed to wait until the Gardener-scheduler deployment is scaled up again to %d: '%v'", *gardenerSchedulerReplicaCount, err)
	}
	return nil
}

var _ = Describe("Shoot Maintenance testing", func() {
	var (
		shootGardenerTest                 *ShootGardenerTest
		intialShootForCreation            v1beta1.Shoot
		shootMaintenanceTestLogger        *logrus.Logger
		shootCleanupNeeded                bool
		cloudProfileCleanupNeeded         bool
		testMachineImageVersion           = "0.0.1-beta"
		testKubernetesVersion             = gardenv1beta1.KubernetesVersion{Version: "0.0.1"}
		testHighestPatchKubernetesVersion = gardenv1beta1.KubernetesVersion{Version: "0.0.5"}
		expirationDateInTheFuture         = metav1.Time{Time: time.Now().Add(time.Second * 20)}
		testMachineImage                  = gardenv1beta1.ShootMachineImage{
			Version: testMachineImageVersion,
		}
		falseVar = false
		trueVar  = true
	)

	CBeforeSuite(func(ctx context.Context) {
		validateFlags()

		shootMaintenanceTestLogger = logger.AddWriter(logger.NewLogger(*logLevel), GinkgoWriter)
		// parse shoot yaml into shoot object and generate random test names for shoots
		_, shootObject, err := CreateShootTestArtifacts(*shootTestYamlPath, *testShootsPrefix, falseVar)
		Expect(err).To(BeNil())

		intialShootForCreation = *shootObject.DeepCopy()

		shootGardenerTest, err = NewShootGardenerTest(*kubeconfig, shootObject, shootMaintenanceTestLogger)
		Expect(err).To(BeNil())

		shootMaintenanceTest, err = NewShootMaintenanceTest(ctx, shootGardenerTest, shootMachineImageName)
		Expect(err).To(BeNil())

		if testMachineryRun != nil && *testMachineryRun {
			shootMaintenanceTestLogger.Info("Running in test Machinery")
			err := setupEnvironmentForMaintenanceTest()
			Expect(err).To(BeNil())
			shootMaintenanceTestLogger.Info("Environment for test-machinery run is prepared")
		}

		// the test machine version is being added to
		testMachineImage.Name = shootMaintenanceTest.ShootMachineImage.Name

		// setup cloud profile for integration test
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

		// get kubernetes versions from cloud profile
		versions, err := helper.GetKubernetesVersionsFromCloudProfile(*shootMaintenanceTest.CloudProfile)
		Expect(err).To(BeNil())

		// add my test kubernetes versions (one low patch version, one high patch version)
		updatedKubernetesVersions := append(versions, testKubernetesVersion, testHighestPatchKubernetesVersion)

		// add test kubernetes version
		err = helper.SetKubernetesVersions(shootMaintenanceTest.CloudProfile, updatedKubernetesVersions, []string{})

		// update Cloud Profile with integration test machineImage & kubernetes version
		cloudProfile, err := shootGardenerTest.GardenClient.Garden().GardenV1beta1().CloudProfiles().Update(shootMaintenanceTest.CloudProfile)
		Expect(err).To(BeNil())
		Expect(cloudProfile).NotTo(BeNil())
		cloudProfileCleanupNeeded = true
	}, InitializationTimeout)

	CAfterSuite(func(ctx context.Context) {
		if cloudProfileCleanupNeeded {
			// retrieve the cloud profile because the machine images & the kubernetes version might got changed during test execution
			err := shootMaintenanceTest.CleanupCloudProfile(ctx, testMachineImage, []gardenv1beta1.KubernetesVersion{testKubernetesVersion, testHighestPatchKubernetesVersion})
			Expect(err).NotTo(HaveOccurred())
			shootMaintenanceTestLogger.Infof("Cleaned Cloud Profile '%s'", shootMaintenanceTest.CloudProfile.Name)
		}
		if testMachineryRun != nil && *testMachineryRun {
			err := restoreEnvironment()
			Expect(err).NotTo(HaveOccurred())
			shootMaintenanceTestLogger.Infof("Environment is restored")
		}
	}, InitializationTimeout)

	CAfterEach(func(ctx context.Context) {
		if shootCleanupNeeded {
			// Finally we delete the shoot again
			shootMaintenanceTestLogger.Infof("Delete shoot %s", shootMaintenanceTest.ShootGardenerTest.Shoot.Name)
			err := shootGardenerTest.DeleteShoot(ctx)
			Expect(err).NotTo(HaveOccurred())
			shootCleanupNeeded = false
		}
	}, WaitForCreateDeleteTimeout)

	CBeforeEach(func(ctx context.Context) {
		if !shootCleanupNeeded {
			// set dummy kubernetes version to shoot
			intialShootForCreation.Spec.Kubernetes.Version = testKubernetesVersion.Version
			// set integration test machineImage to shoot
			updateImage := helper.UpdateDefaultMachineImage(shootMaintenanceTest.CloudProvider, &testMachineImage)
			Expect(updateImage).NotTo(BeNil())
			updateImage(&intialShootForCreation.Spec.Cloud)

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
		integrationTestShoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = &falseVar
		integrationTestShoot.Annotations[common.ShootOperation] = common.ShootOperationMaintain

		// update integration test shoot
		err = shootMaintenanceTest.TryUpdateShootForMachineImageMaintenance(ctx, integrationTestShoot, false, nil)
		Expect(err).To(BeNil())

		err = shootMaintenanceTest.WaitForExpectedMachineImageMaintenance(ctx, testMachineImage, shootMaintenanceTest.CloudProvider, false, time.Now().Add(time.Second*10))
		Expect(err).To(BeNil())

		By("AutoUpdate.MachineImageVersion == true && expirationDate does not apply -> shoot machineImage must be updated in maintenance time")
		// set test specific shoot settings
		integrationTestShoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = &trueVar
		integrationTestShoot.Annotations[common.ShootOperation] = common.ShootOperationMaintain

		// update integration test shoot - set maintain now annotation & autoupdate == true
		err = shootMaintenanceTest.TryUpdateShootForMachineImageMaintenance(ctx, integrationTestShoot, false, nil)
		Expect(err).To(BeNil())

		err = shootMaintenanceTest.WaitForExpectedMachineImageMaintenance(ctx, shootMaintenanceTest.ShootMachineImage, shootMaintenanceTest.CloudProvider, true, time.Now().Add(time.Second*20))
		Expect(err).To(BeNil())

		By("AutoUpdate.MachineImageVersion == default && expirationDate does not apply -> shoot machineImage must be updated in maintenance time")
		// set test specific shoot settings
		integrationTestShoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = nil
		integrationTestShoot.Annotations[common.ShootOperation] = common.ShootOperationMaintain

		// reset machine image from latest version to dummy version
		updateImage := helper.UpdateMachineImages(shootMaintenanceTest.CloudProvider, []*gardenv1beta1.ShootMachineImage{&testMachineImage})
		Expect(updateImage).NotTo(BeNil())

		// update integration test shoot - downgrade image again & set maintain now  annotation & autoupdate == nil (default)
		err = shootMaintenanceTest.TryUpdateShootForMachineImageMaintenance(ctx, integrationTestShoot, true, updateImage)
		Expect(err).To(BeNil())

		err = shootMaintenanceTest.WaitForExpectedMachineImageMaintenance(ctx, shootMaintenanceTest.ShootMachineImage, shootMaintenanceTest.CloudProvider, true, time.Now().Add(time.Second*20))
		Expect(err).To(BeNil())

		By("AutoUpdate.MachineImageVersion == false && expirationDate applies -> shoot machineImage must be updated in maintenance time")
		defer func() {
			// make sure to remove expiration date from cloud profile after test
			err = shootMaintenanceTest.TryUpdateCloudProfileForMaintenance(ctx, shootMaintenanceTest.ShootGardenerTest.Shoot, testMachineImage, nil)
			Expect(err).To(BeNil())
			shootMaintenanceTestLogger.Infof("Cleaned expiration date on machine image from Cloud Profile '%s'", shootMaintenanceTest.CloudProfile.Name)
		}()

		// set test specific shoot settings
		integrationTestShoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = &falseVar

		// reset machine image from latest version to dummy version
		updateImage = helper.UpdateMachineImages(shootMaintenanceTest.CloudProvider, []*gardenv1beta1.ShootMachineImage{&testMachineImage})
		Expect(updateImage).NotTo(BeNil())

		// update integration test shoot - downgrade image again & set maintain now  annotation & autoupdate == nil (default)
		err = shootMaintenanceTest.TryUpdateShootForMachineImageMaintenance(ctx, integrationTestShoot, true, updateImage)
		Expect(err).To(BeNil())

		// modify cloud profile for test
		err = shootMaintenanceTest.TryUpdateCloudProfileForMaintenance(ctx, shootMaintenanceTest.ShootGardenerTest.Shoot, testMachineImage, &expirationDateInTheFuture)
		Expect(err).To(BeNil())

		//sleep so that expiration date is in the past - forceUpdate is required
		time.Sleep(30 * time.Second)
		integrationTestShoot.Annotations[common.ShootOperation] = common.ShootOperationMaintain

		// update integration test shoot - set maintain now  annotation
		err = shootMaintenanceTest.TryUpdateShootForMachineImageMaintenance(ctx, integrationTestShoot, false, nil)
		Expect(err).To(BeNil())

		err = shootMaintenanceTest.WaitForExpectedMachineImageMaintenance(ctx, shootMaintenanceTest.ShootMachineImage, shootMaintenanceTest.CloudProvider, true, time.Now().Add(time.Minute*1))
		Expect(err).To(BeNil())

	}, WaitForCreateDeleteTimeout)

	CIt("Maintenance test - Kubernetes Version force upgrade", func(ctx context.Context) {
		By("AutoUpdate.KubernetesVersion == false && expirationDate does not apply -> shoot Kubernetes version must not be updated in maintenance time")
		integrationTestShoot, err := shootGardenerTest.GetShoot(ctx)
		Expect(err).To(BeNil())

		// set test specific shoot settings
		integrationTestShoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = falseVar
		integrationTestShoot.Annotations[common.ShootOperation] = common.ShootOperationMaintain

		// update integration test shoot
		err = shootMaintenanceTest.TryUpdateShootForKubernetesMaintenance(ctx, integrationTestShoot)
		Expect(err).To(BeNil())

		err = shootMaintenanceTest.WaitForExpectedKubernetesVersionMaintenance(ctx, testKubernetesVersion.Version, false, time.Now().Add(time.Second*20))
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
		integrationTestShoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = falseVar

		// update integration test shoot - autoupdate == false
		err = shootMaintenanceTest.TryUpdateShootForKubernetesMaintenance(ctx, integrationTestShoot)
		Expect(err).To(BeNil())

		//sleep so that expiration date is in the past - forceUpdate is required
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
