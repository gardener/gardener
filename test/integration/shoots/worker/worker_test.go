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
		- Tests the creation of a shoot specifying multiple workers with different machine images

	Prerequisites
		- CloudProfile for Shoot must have at least two MachineImages. Will default two the first two images if no image names are provided via flags (machine-image-name & machine-image-name-2).
		- In Garden cluster: ControllerRegistration for CloudProvider must be configured (provider config) to map the machine image name & version in the CloudProfile to the machine image ami. This is to configure the WorkerController.

	BeforeSuite
		- Modify Shoot to have multiple machine images
		- Create Shoot

	AfterSuite
		- Delete Shoot

	Test: Shoot nodes should have different Image Versions
	Expected Output
		- Shoot has nodes with the machine image names specified in the shoot spec (node.Status.NodeInfo.OSImage)
 **/

package worker

import (
	"context"
	"flag"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/test/integration/framework"
	. "github.com/gardener/gardener/test/integration/shoots"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	logLevel               = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG")
	kubeconfig             = flag.String("kubecfg", "", "the path to the kubeconfig of Garden cluster that will be used for integration tests")
	testShootsPrefix       = flag.String("prefix", "", "prefix to use for test shoots")
	testShootName          = flag.String("shoot-name", "", "unique name to use for test shoots. Used by test-machinery.")
	projectNamespace       = flag.String("project-namespace", "", "project namespace where the shoot will be created")
	shootMachineImageName  = flag.String("machine-image-name", "", "the Machine Image Name of the first worker of the test shoot.")
	shootMachineImageName2 = flag.String("machine-image-name-2", "", "the Machine Image Name of the second worker of the test shoot.")
	cloudProfile           = flag.String("cloud-profile", "", "cloudProfile to use for the shoot")
	shootRegion            = flag.String("region", "", "region to use for the shoot. Must be compatible with the infrastructureProvider.Zone.")
	secretBinding          = flag.String("secret-binding", "", "the secretBinding for the provider account of the shoot")
	shootProviderType      = flag.String("provider-type", "", "the type of the cloud provider where the shoot is deployed to. e.g gcp, aws,azure,alicloud")
	shootK8sVersion        = flag.String("k8s-version", "", "kubernetes version to use for the shoot")
	externalDomain         = flag.String("external-domain", "", "external domain to use for the shoot. If not set, will use the default domain.")
	workerZone             = flag.String("worker-zone", "", "zone to use for every worker of the shoot.")
	networkingPods         = flag.String("networking-pods", "", "the spec.networking.pods to use for this shoot. Optional.")
	networkingServices     = flag.String("networking-services", "", "the spec.networking.services to use for this shoot. Optional.")
	networkingNodes        = flag.String("networking-nodes", "", "the spec.networking.nodes to use for this shoot. Optional.")

	// ProviderConfigs flags
	infrastructureProviderConfig = flag.String("infrastructure-provider-config-filepath", "", "filepath to the provider specific infrastructure config")
	controlPlaneProviderConfig   = flag.String("controlplane-provider-config-filepath", "", "filepath to the provider specific infrastructure config")
	networkingProviderConfig     = flag.String("networking-provider-config-filepath", "", "filepath to the provider specific infrastructure config")
	workersConfig                = flag.String("workers-config-filepath", "", "filepath to the workers config.")

	// other
	shootYamlPath       = "/example/90-shoot.yaml"
	gardenTestOperation *GardenerTestOperation
	workerGardenerTest  *WorkerGardenerTest
	shootGardenerTest   *ShootGardenerTest
	workerTestLogger    *logrus.Logger
)

const (
	InitializationTimeout = 15 * time.Minute
	TearDownTimeout       = 5 * time.Minute
	DumpStateTimeout      = 5 * time.Minute
)

func validateFlags() {
	if !StringSet(*kubeconfig) {
		Fail("you need to specify the correct path for the kubeconfig")
	}

	if !FileExists(*kubeconfig) {
		Fail("kubeconfig path does not exist")
	}

	if !StringSet(*infrastructureProviderConfig) {
		Fail(fmt.Sprintf("you need to specify the filepath to the infrastructureProviderConfig for the provider '%s'", *shootProviderType))
	}

	if !FileExists(*infrastructureProviderConfig) {
		Fail("path to the infrastructureProviderConfig of the Shoot is invalid")
	}

	if StringSet(*controlPlaneProviderConfig) {
		if !FileExists(*controlPlaneProviderConfig) {
			Fail("path to the controlPlaneProviderConfig of the Shoot is invalid")
		}
	}

	if StringSet(*networkingProviderConfig) {
		if !FileExists(*networkingProviderConfig) {
			Fail("path to the networkingProviderConfig of the Shoot is invalid")
		}
	}

	if !StringSet(*projectNamespace) {
		Fail("you need to specify projectNamespace")
	}
}

var _ = Describe("Worker Suite", func() {
	var cleanupRequired bool

	CBeforeSuite(func(ctx context.Context) {
		validateFlags()

		workerTestLogger = logger.AddWriter(logger.NewLogger(*logLevel), GinkgoWriter)

		shoot := prepareShoot()

		shootGardenerTest, err := NewShootGardenerTest(*kubeconfig, shoot, workerTestLogger)
		Expect(err).NotTo(HaveOccurred())

		workerGardenerTest, err = NewWorkerGardenerTest(shootGardenerTest)
		Expect(err).NotTo(HaveOccurred())

		if len(*workersConfig) == 0 {
			err := workerGardenerTest.SetupShootWorkers(shootMachineImageName, shootMachineImageName2, workerZone)
			Expect(err).NotTo(HaveOccurred())
		}

		shootObject, err := shootGardenerTest.CreateShoot(ctx)
		if err != nil {
			if shootObject != nil {
				if err := gardenTestOperation.AddShoot(ctx, shootObject); err != nil {
					workerTestLogger.Errorf("Cannot add shoot %s to test operation: %s", shoot.Name, err.Error())
				}
				gardenTestOperation.DumpState(ctx)
			}
			workerTestLogger.Fatalf("Cannot create shoot %s: %s", shoot.Name, err.Error())
		}
		workerTestLogger.Infof("Successfully created shoot %s", shoot.Name)
		cleanupRequired = true

		gardenTestOperation, err = NewGardenTestOperationWithShoot(ctx, shootGardenerTest.GardenClient, workerTestLogger, shoot)
		Expect(err).NotTo(HaveOccurred())

		workerGardenerTest.ShootClient = gardenTestOperation.ShootClient

	}, InitializationTimeout)

	CAfterSuite(func(ctx context.Context) {
		By("Deleting the shoot")
		if cleanupRequired {
			err := shootGardenerTest.DeleteShoot(ctx)
			Expect(err).NotTo(HaveOccurred())
		}
	}, InitializationTimeout)

	CAfterEach(func(ctx context.Context) {
		gardenTestOperation.AfterEach(ctx)
	}, DumpStateTimeout)

	CIt("Shoot nodes should have different Image Versions", func(ctx context.Context) {
		By(fmt.Sprintf("Checking if shoot is compatible for testing"))

		nodesList, err := gardenTestOperation.ShootClient.Kubernetes().CoreV1().Nodes().List(metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())

		areThereTwoDifferentNodes := false
		firstNode := nodesList.Items[0]

		for i := 1; i < len(nodesList.Items); i++ {
			if firstNode.Status.NodeInfo.OSImage != nodesList.Items[i].Status.NodeInfo.OSImage {
				areThereTwoDifferentNodes = true
				break
			}
		}

		Expect(areThereTwoDifferentNodes).To(Equal(true))
	}, TearDownTimeout)
})

// prepareShoot parses the shoot.yaml from the given path and sets the shoot information provided by the flags
func prepareShoot() *gardencorev1beta1.Shoot {
	// if running in test machinery, test will be executed from root of the project
	if !FileExists(fmt.Sprintf(".%s", shootYamlPath)) {
		// locally, we need find the example shoot
		shootYamlPath = GetProjectRootPath() + shootYamlPath
		Expect(FileExists(shootYamlPath)).To(Equal(true))
	}

	// Create Shoot Object
	_, shootObject, err := CreateShootTestArtifacts(shootYamlPath, testShootsPrefix, projectNamespace, shootRegion, cloudProfile, secretBinding, shootProviderType, shootK8sVersion, externalDomain, true, true)
	Expect(err).To(BeNil())

	if testShootName != nil && len(*testShootName) > 0 {
		shootObject.Name = *testShootName
	}

	if networkingPods != nil && len(*networkingPods) > 0 {
		shootObject.Spec.Networking.Pods = networkingPods
	}

	if networkingServices != nil && len(*networkingServices) > 0 {
		shootObject.Spec.Networking.Services = networkingServices
	}

	if networkingNodes != nil && len(*networkingNodes) > 0 {
		shootObject.Spec.Networking.Nodes = networkingNodes
	}

	// set ProviderConfigs
	err = SetProviderConfigsFromFilepath(shootObject, infrastructureProviderConfig, controlPlaneProviderConfig, networkingProviderConfig, workersConfig)
	Expect(err).To(BeNil())
	return shootObject
}
