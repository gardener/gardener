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
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"

	helper "github.com/gardener/gardener/.test-defs/cmd"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/test/integration/framework"
)

var (
	shootName         string
	projectNamespace  string
	kubeconfigPath    string
	cloudprovider     gardenv1beta1.CloudProvider
	cloudprofileName  string
	secretBindingName string
	region            string
	zone              string
	k8sVersion        string

	// optional parameters
	shootArtifactPath string
	machineType       string
	autoScalerMin     *int
	autoScalerMax     *int

	// required for openstack
	floatingPoolName     string
	loadBalancerProvider string

	testLogger *logrus.Logger

	testMachineImageVersion = "0.0.1-beta"

	testMachineImage = gardenv1beta1.ShootMachineImage{
		Version: testMachineImageVersion,
	}

	shootMaintenanceTest          *framework.ShootMaintenanceTest
	gardenerSchedulerReplicaCount *int32

	setupShootMaintenanceCtx = context.Background()

	// timeouts
	mainContextTimeout  = time.Minute * 5
	setupContextTimeout = time.Minute * 2
	restoreCtxTimeout   = time.Minute * 2
)

func init() {
	testLogger = logger.NewLogger("debug")

	setEnvironmentVariables()
	setupShootMaintenanceTest()
}

func setupShootMaintenanceTest() {
	defer setupShootMaintenanceCtx.Done()

	gardenerConfigPath := fmt.Sprintf("%s/gardener.config", kubeconfigPath)
	shootGardenerTest, err := framework.NewShootGardenerTest(gardenerConfigPath, nil, testLogger)
	if err != nil {
		testLogger.Fatalf("Cannot create ShootGardenerTest %s", err.Error())
	}
	_, shootObject, err := framework.CreateShootTestArtifacts(shootArtifactPath, "", true)
	if err != nil {
		testLogger.Fatalf("Cannot create shoot artifact %s", err.Error())
	}
	updateShootWithEnvVars(shootObject)

	maintenanceTest, err := framework.NewShootMaintenanceTest(setupShootMaintenanceCtx, shootGardenerTest)
	if err != nil {
		testLogger.Fatalf("Failed to create an new Shoot Maintenance test '%v'", err)
	}
	shootMaintenanceTest = maintenanceTest
}

// setup the integration test environment by manipulation the Gardener Components (namespace garden) in the garden cluster
// scale down the gardener-scheduler to 0 replicas
func setupEnvironmentForMaintenanceTest() error {
	ctxSetup, cancelCtxSetup := context.WithTimeout(context.Background(), setupContextTimeout)
	replicas, err := framework.GetDeploymentReplicas(ctxSetup, shootMaintenanceTest.ShootGardenerTest.GardenClient.Client(), "garden", "gardener-scheduler")
	if err != nil {
		return fmt.Errorf("failed to retrieve the replica count of the Gardener-scheduler deployment: '%v'", err)
	}
	if replicas == nil || *replicas == 0 {
		return nil
	}
	gardenerSchedulerReplicaCount = replicas

	defer cancelCtxSetup()

	// scale down the scheduler deployment
	if err := kubernetes.ScaleDeployment(ctxSetup, shootMaintenanceTest.ShootGardenerTest.GardenClient.Client(), kutil.Key("garden", "gardener-scheduler"), 0); err != nil {
		return fmt.Errorf("failed to scale down the replica count of the Gardener-scheduler deployment: '%v'", err)
	}

	// wait until scaled down
	if err := framework.WaitUntilDeploymentScaled(ctxSetup, shootMaintenanceTest.ShootGardenerTest.GardenClient.Client(), "garden", "gardener-scheduler", 0); err != nil {
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
	if err := framework.WaitUntilDeploymentScaled(ctxRestore, shootMaintenanceTest.ShootGardenerTest.GardenClient.Client(), "garden", "gardener-scheduler", *gardenerSchedulerReplicaCount); err != nil {
		return fmt.Errorf("failed to wait until the Gardener-scheduler deployment is scaled up again to %d: '%v'", *gardenerSchedulerReplicaCount, err)
	}
	return nil
}

func setEnvironmentVariables() {
	shootName = os.Getenv("SHOOT_NAME")
	if shootName == "" {
		testLogger.Fatalf("EnvVar 'SHOOT_NAME' needs to be specified")
	}
	projectNamespace = os.Getenv("PROJECT_NAMESPACE")
	if projectNamespace == "" {
		testLogger.Fatalf("EnvVar 'PROJECT_NAMESPACE' needs to be specified")
	}
	kubeconfigPath = os.Getenv("TM_KUBECONFIG_PATH")
	if kubeconfigPath == "" {
		testLogger.Fatalf("EnvVar 'TM_KUBECONFIG_PATH' needs to be specified")
	}
	cloudprovider = gardenv1beta1.CloudProvider(os.Getenv("CLOUDPROVIDER"))
	if cloudprovider == "" {
		testLogger.Fatalf("EnvVar 'CLOUDPROVIDER' needs to be specified")
	}
	cloudprofileName = os.Getenv("CLOUDPROFILE")
	if cloudprofileName == "" {
		testLogger.Fatalf("EnvVar 'CLOUDPROFILE' needs to be specified")
	}
	secretBindingName = os.Getenv("SECRET_BINDING")
	if secretBindingName == "" {
		testLogger.Fatalf("EnvVar 'SECRET_BINDING' needs to be specified")
	}
	region = os.Getenv("REGION")
	if region == "" {
		testLogger.Fatalf("EnvVar 'REGION' needs to be specified")
	}
	zone = os.Getenv("ZONE")
	if zone == "" && cloudprovider != gardenv1beta1.CloudProviderAzure {
		testLogger.Fatalf("EnvVar 'ZONE' needs to be specified")
	}
	k8sVersion = os.Getenv("K8S_VERSION")
	if k8sVersion == "" {
		testLogger.Fatalf("EnvVar 'K8S_VERSION' needs to be specified")
	}
	shootArtifactPath = os.Getenv("SHOOT_ARTIFACT_PATH")
	if shootArtifactPath == "" {
		shootArtifactPath = fmt.Sprintf("example/90-shoot-%s.yaml", cloudprovider)
	}
	machineType = os.Getenv("MACHINE_TYPE")
	if autoScalerMinEnv := os.Getenv("AUTOSCALER_MIN"); autoScalerMinEnv != "" {
		autoScalerMinInt, err := strconv.Atoi(autoScalerMinEnv)
		if err != nil {
			testLogger.Infof("Cannot parse %s: %s", autoScalerMinEnv, err.Error())
		}
		autoScalerMin = &autoScalerMinInt
	}
	if autoScalerMaxEnv := os.Getenv("AUTOSCALER_MAX"); autoScalerMaxEnv != "" {
		autoScalerMaxInt, err := strconv.Atoi(autoScalerMaxEnv)
		if err != nil {
			testLogger.Infof("Cannot parse %s: %s", autoScalerMaxEnv, err.Error())
		}
		autoScalerMax = &autoScalerMaxInt
	}
	loadBalancerProvider = os.Getenv("LOADBALANCER_PROVIDER")
	floatingPoolName = os.Getenv("FLOATING_POOL_NAME")
	if cloudprovider == gardenv1beta1.CloudProviderOpenStack && floatingPoolName == "" {
		testLogger.Fatalf("EnvVar 'FLOATING_POOL_NAME' needs to be specified when creating a shoot on openstack")
	}
}

func main() {
	if errs := executeTest(); len(errs) != 0 {
		testLogger.Fatalf("Test execution failed: %v", errs)
	}
	testLogger.Info("Test execution successful")
}

func executeTest() (errs []error) {
	ctxMain, cancelCtxMain := context.WithTimeout(context.Background(), mainContextTimeout)

	defer cancelCtxMain()
	defer func() {
		if err := restoreEnvironment(); err != nil {
			errs = append(errs, err)
		}
	}()

	if err := setupEnvironmentForMaintenanceTest(); err != nil {
		return append(errs, err)
	}

	found, image, err := v1beta1helper.DetermineMachineImageForName(*shootMaintenanceTest.CloudProfile, shootMaintenanceTest.ShootMachineImage.Name)
	if err != nil {
		return append(errs, fmt.Errorf("failed to determine machine image for name: '%s'", err.Error()))
	}
	if !found {
		return append(errs, fmt.Errorf("failed to determine machine image for name: '%s'", shootMaintenanceTest.ShootMachineImage.Name))
	}

	// test machine image needs to have the same name but a lower version
	testMachineImage.Name = shootMaintenanceTest.ShootMachineImage.Name
	imageVersions := append(image.Versions, gardenv1beta1.MachineImageVersion{Version: testMachineImageVersion})

	// setup cloud profile & shoot for integration test
	cloudProfileImages, err := v1beta1helper.GetMachineImagesFromCloudProfile(shootMaintenanceTest.CloudProfile)
	if err != nil {
		return append(errs, fmt.Errorf("failed to determine cloud profile images: '%s'", err.Error()))
	}
	if cloudProfileImages == nil {
		return append(errs, fmt.Errorf("cloud profile does not contain any machine images"))
	}
	updatedCloudProfileImages, err := v1beta1helper.SetMachineImageVersionsToMachineImage(cloudProfileImages, shootMaintenanceTest.ShootMachineImage.Name, imageVersions)

	if err := v1beta1helper.SetMachineImages(shootMaintenanceTest.CloudProfile, updatedCloudProfileImages); err != nil {
		return append(errs, fmt.Errorf("failed to set machine images for cloud provider: '%s'", err.Error()))
	}
	// update Cloud Profile with integration test machineImage
	_, err = shootMaintenanceTest.ShootGardenerTest.GardenClient.Garden().GardenV1beta1().CloudProfiles().Update(shootMaintenanceTest.CloudProfile)
	if err != nil {
		return append(errs, fmt.Errorf("failed to update Cloud Profile with integration test machineImage '%v'", err))
	}

	defer func() {
		if err := cleanupCloudProfile(ctxMain, shootMaintenanceTest); err != nil {
			errs = append(errs, err)
		}
	}()

	// set integration test machineImage to shoot
	updateImage := v1beta1helper.UpdateDefaultMachineImage(shootMaintenanceTest.CloudProvider, &testMachineImage)
	if err != nil {
		return append(errs, fmt.Errorf("failed to update Machine Image on shoot: '%v'", err))
	}
	updateImage(&shootMaintenanceTest.ShootGardenerTest.Shoot.Spec.Cloud)

	_, err = shootMaintenanceTest.CreateShoot(ctxMain)
	if err != nil {
		return append(errs, fmt.Errorf("failed to create shoot resource: '%v'", err))
	}

	defer func() {
		if err := shootMaintenanceTest.ShootGardenerTest.DeleteShoot(ctxMain); err != nil {
			errs = append(errs, err)
		}
	}()

	if err := testMachineImageMaintenance(ctxMain, shootMaintenanceTest.ShootGardenerTest, shootMaintenanceTest, shootMaintenanceTest.ShootGardenerTest.Shoot); err != nil {
		return append(errs, err)
	}
	return errs
}

func updateShootWithEnvVars(shootObject *gardenv1beta1.Shoot) {
	shootObject.Name = shootName
	shootObject.Namespace = projectNamespace
	shootObject.Spec.Cloud.Profile = cloudprofileName
	shootObject.Spec.Cloud.Region = region
	shootObject.Spec.Cloud.SecretBindingRef.Name = secretBindingName
	shootObject.Spec.Kubernetes.Version = k8sVersion
	helper.UpdateAnnotations(shootObject)
	if err := helper.UpdateWorkerZone(shootObject, cloudprovider, zone); err != nil {
		testLogger.Warnf(err.Error())
	}
	if err := helper.UpdateMachineType(shootObject, cloudprovider, machineType); err != nil {
		testLogger.Warnf(err.Error())
	}
	if err := helper.UpdateAutoscalerMin(shootObject, cloudprovider, autoScalerMin); err != nil {
		testLogger.Warnf(err.Error())
	}
	if err := helper.UpdateAutoscalerMax(shootObject, cloudprovider, autoScalerMax); err != nil {
		testLogger.Warnf(err.Error())
	}
	helper.UpdateFloatingPoolName(shootObject, floatingPoolName, cloudprovider)
	helper.UpdateLoadBalancerProvider(shootObject, loadBalancerProvider, cloudprovider)
}

func cleanupCloudProfile(ctx context.Context, shootMaintenanceTest *framework.ShootMaintenanceTest) error {
	// retrieve the cloud profile because the machine images might got changed during test execution
	err := shootMaintenanceTest.RemoveTestMachineImageVersionFromCloudProfile(ctx, testMachineImage)
	if err != nil {
		return fmt.Errorf("failed to cleanup CloudProfile after integration test: '%v'", err)
	}
	logger.Logger.Infof("cleaned Cloud Profile '%s'", shootMaintenanceTest.CloudProfile.Name)
	return nil
}
