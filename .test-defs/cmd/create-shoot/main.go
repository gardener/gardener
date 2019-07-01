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
	"github.com/ghodss/yaml"
	"os"
	"strconv"

	helper "github.com/gardener/gardener/.test-defs/cmd"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/test/integration/framework"
	"github.com/sirupsen/logrus"
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
	seed                string
	machineImage        string
	machineImageVersion string
	shootArtifactPath   string
	machineType         string
	autoScalerMin       *int
	autoScalerMax       *int

	// required for openstack
	floatingPoolName     string
	loadBalancerProvider string

	testLogger *logrus.Logger
)

func init() {
	testLogger = logger.NewLogger("debug")

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
	seed = os.Getenv("SEED")
	machineImage = os.Getenv("MACHINE_IMAGE")
	machineImageVersion = os.Getenv("MACHINE_IMAGE_VERSION")
	if machineImage != "" && machineImageVersion == "" {
		testLogger.Fatalf("MACHINE_IMAGE_VERSION has to be defined if MACHINE_IMAGE is set")
	}
	if machineImageVersion != "" && machineImage == "" {
		testLogger.Fatalf("MACHINE_IMAGE has to be defined if MACHINE_IMAGE_VERSION is set")
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
	ctx := context.Background()
	defer ctx.Done()
	gardenerConfigPath := fmt.Sprintf("%s/gardener.config", kubeconfigPath)

	shootGardenerTest, err := framework.NewShootGardenerTest(gardenerConfigPath, nil, testLogger)
	if err != nil {
		testLogger.Fatalf("Cannot create ShootGardenerTest %s", err.Error())
	}

	_, shootObject, err := framework.CreateShootTestArtifacts(shootArtifactPath, "", true)
	if err != nil {
		testLogger.Fatalf("Cannot create shoot artifact %s", err.Error())
	}

	shootObject.Name = shootName
	shootObject.Namespace = projectNamespace
	shootObject.Spec.Cloud.Profile = cloudprofileName
	shootObject.Spec.Cloud.Region = region
	shootObject.Spec.Cloud.SecretBindingRef.Name = secretBindingName
	shootObject.Spec.Kubernetes.Version = k8sVersion

	if seed != "" {
		shootObject.Spec.Cloud.Seed = &seed
	}
	helper.UpdateAnnotations(shootObject)
	if err := helper.UpdateMachineImage(shootObject, cloudprovider, machineImage, machineImageVersion); err != nil {
		testLogger.Warnf(err.Error())
	}
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

	// TODO: tests need to be adopted when nginx gets removed.
	shootObject.Spec.Addons.NginxIngress = &gardenv1beta1.NginxIngress{
		Addon: gardenv1beta1.Addon{
			Enabled: true,
		},
	}

	testLogger.Infof("Create shoot %s in namespace %s", shootName, projectNamespace)
	if err := printShoot(shootObject); err != nil {
		testLogger.Fatalf("Cannot decode shoot %s: %s", shootName, err)
	}

	shootGardenerTest.Shoot = shootObject
	shootObject, err = shootGardenerTest.CreateShoot(ctx)
	if err != nil {
		testLogger.Fatalf("Cannot create shoot %s: %s", shootName, err.Error())
	}
	testLogger.Infof("Successfully created shoot %s", shootName)

	backupInfrastructure, err := helper.GetBackupInfrastructureOfShoot(ctx, shootGardenerTest, shootObject)
	if err != nil {
		testLogger.Fatal(err)
	}
	helper.UpdateBackupInfrastructureAnnotations(backupInfrastructure)
	if err := shootGardenerTest.GardenClient.Client().Update(ctx, backupInfrastructure); err != nil {
		testLogger.Fatalf("Cannot update annotation of backupinfrastructure %s: %s", backupInfrastructure.Name, err.Error())
	}

	shootTestOperations, err := framework.NewGardenTestOperation(ctx, shootGardenerTest.GardenClient, testLogger, shootObject)
	if err != nil {
		testLogger.Fatalf("Cannot create shoot %s: %s", shootName, err.Error())
	}

	err = shootTestOperations.DownloadKubeconfig(ctx, shootTestOperations.SeedClient, shootTestOperations.ShootSeedNamespace(), gardenv1beta1.GardenerName, fmt.Sprintf("%s/shoot.config", kubeconfigPath))
	if err != nil {
		testLogger.Fatalf("Cannot download shoot kubeconfig: %s", err.Error())
	}
	err = shootTestOperations.DownloadKubeconfig(ctx, shootTestOperations.GardenClient, shootTestOperations.Seed.Spec.SecretRef.Namespace, shootTestOperations.Seed.Spec.SecretRef.Name, fmt.Sprintf("%s/seed.config", kubeconfigPath))
	if err != nil {
		testLogger.Fatalf("Cannot download seed kubeconfig: %s", err.Error())
	}
	testLogger.Infof("Finished creating shoot %s", shootName)
}

func printShoot(shoot *gardenv1beta1.Shoot) error {
	d, err := yaml.Marshal(shoot)
	if err != nil {
		return err
	}
	fmt.Print(string(d))
	return nil
}
