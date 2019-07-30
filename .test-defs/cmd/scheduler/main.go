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

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/scheduler/apis/config"
	"github.com/gardener/gardener/test/integration/framework"
	"github.com/sirupsen/logrus"
	helper "github.com/gardener/gardener/.test-defs/cmd"
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
	floatingPoolName string
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
	hostConfigPath := fmt.Sprintf("%s/host.config", kubeconfigPath)
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

	shootGardenerTest.Shoot = shootObject

	schedulerGardenerTest, err := framework.NewGardenSchedulerTest(ctx, shootGardenerTest, hostConfigPath)
	if err != nil {
		testLogger.Fatalf("Failed to create an new Gardener Scheduler test '%v'", err)
	}
	if schedulerGardenerTest.SchedulerConfiguration.Strategy == config.SameRegion {
		testLogger.Infof("Executing Scheduler Test for strategy '%s'", config.SameRegion)
		SameRegionTest(ctx, schedulerGardenerTest)
	} else if schedulerGardenerTest.SchedulerConfiguration.Strategy == config.MinimalDistance {
		testLogger.Infof("Executing Scheduler Test for strategy '%s'", config.MinimalDistance)
		MinimalDistanceTest(ctx, schedulerGardenerTest)
		ApiServerBindingTestWrongSchedulerDecision(ctx, schedulerGardenerTest)
	} else {
		testLogger.Infof("No Scheduler Tests are being executed. Unknown Strategy: '%s'", schedulerGardenerTest.SchedulerConfiguration.Strategy)
	}
}
