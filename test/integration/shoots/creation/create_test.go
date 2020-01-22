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
		- Tests the creation of a shoot

	BeforeSuite
		- Parse Shoot from example folder and provided flags

	Test: Shoot creation
	Expected Output
		- Successful reconciliation after Shoot creation
 **/

package creation

import (
	"context"
	"flag"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/test/integration/framework"
	. "github.com/gardener/gardener/test/integration/shoots"

	"github.com/ghodss/yaml"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var (
	// flags
	gardenerKubeconfigPath   = flag.String("gardener-kubecfg-path", "", "the path to the kubeconfig of Garden cluster. File must exist.")
	shootKubeconfigPath      = flag.String("shoot-kubecfg-path", "", "the path to where the Kubeconfig of the Shoot cluster will be downloaded to.")
	seedKubeconfigPath       = flag.String("seed-kubecfg-path", "", "the path to where the Kubeconfig of the Seed cluster will be downloaded to.")
	testShootName            = flag.String("shoot-name", "", "unique name to use for test shoots. Used by test-machinery.")
	testShootPrefix          = flag.String("prefix", "", "prefix for generated shoot name. Usually used locally to auto generate a unique name.")
	logLevel                 = flag.String("verbose", "", "verbosity level, when set, logging level will be DEBUG.")
	shootMachineImageName    = flag.String("machine-image-name", "", "the Machine Image Name of the test shoot. Defaults to first machine image in the CloudProfile.")
	shootMachineType         = flag.String("machine-type", "", "the Machine type of the first worker of the test shoot. Needs to match the machine types for that Provider available in the CloudProfile.")
	shootMachineImageVersion = flag.String("machine-image-version", "", "the Machine Image version of the first worker of the test shoot. Needs to be set when the MachineImageName is set.")
	cloudProfile             = flag.String("cloud-profile", "", "cloudProfile to use for the shoot.")
	seedName                 = flag.String("seed", "", "Name of the seed to use for the shoot.")
	shootRegion              = flag.String("region", "", "region to use for the shoot. Must be compatible with the infrastructureProvider.Zone.")
	secretBinding            = flag.String("secret-binding", "", "the secretBinding for the provider account of the shoot.")
	shootProviderType        = flag.String("provider-type", "", "the type of the cloud provider where the shoot is deployed to. e.g gcp, aws,azure,alicloud.")
	shootK8sVersion          = flag.String("k8s-version", "", "kubernetes version to use for the shoot.")
	projectNamespace         = flag.String("project-namespace", "", "project namespace where the shoot will be created.")
	externalDomain           = flag.String("external-domain", "", "external domain to use for the shoot. If not set, will use the default domain.")
	workerZone               = flag.String("worker-zone", "", "zone to use for every worker of the shoot.")
	networkingPods           = flag.String("networking-pods", "", "the spec.networking.pods to use for this shoot. Optional.")
	networkingServices       = flag.String("networking-services", "", "the spec.networking.services to use for this shoot. Optional.")
	networkingNodes          = flag.String("networking-nodes", "", "the spec.networking.nodes to use for this shoot. Optional.")

	// ProviderConfigs flags
	infrastructureProviderConfig = flag.String("infrastructure-provider-config-filepath", "", "filepath to the provider specific infrastructure config.")
	controlPlaneProviderConfig   = flag.String("controlplane-provider-config-filepath", "", "filepath to the provider specific infrastructure config.")
	networkingProviderConfig     = flag.String("networking-provider-config-filepath", "", "filepath to the provider specific infrastructure config.")
	workersConfig                = flag.String("workers-config-filepath", "", "filepath to the worker config.")

	// other
	shootGardenerTest     *ShootGardenerTest
	testLogger            *logrus.Logger
	shootYamlPath         = "/example/90-shoot.yaml"
	err                   error
	gardenerTestOperation *GardenerTestOperation
)

const (
	CreateAndReconcileTimeout = 2 * time.Hour
	InitializationTimeout     = 20 * time.Second
)

func validateFlags() {
	if !StringSet(*gardenerKubeconfigPath) {
		Fail("you need to specify the correct path for the gardenerKubeconfigPath")
	}

	if !FileExists(*gardenerKubeconfigPath) {
		Fail("gardenerKubeconfigPath path does not exist")
	}

	if !StringSet(*logLevel) {
		level := "debug"
		logLevel = &level
	}

	if !StringSet(*shootProviderType) {
		Fail("you need to specify provider type of the shoot")
	}

	if !StringSet(*projectNamespace) {
		Fail("you need to specify projectNamespace")
	}

	if StringSet(*shootMachineImageName) && !StringSet(*shootMachineImageVersion) {
		Fail("shootMachineImageVersion has to be defined if shootMachineImageName is set")
	}

	if StringSet(*shootMachineImageVersion) && !StringSet(*shootMachineImageName) {
		Fail("shootMachineImageName has to be defined if shootMachineImageVersion is set")
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

	if StringSet(*workersConfig) {
		if !FileExists(*workersConfig) {
			Fail("path to the worker config of the Shoot is invalid")
		}
	}
}

var _ = Describe("Shoot Creation testing", func() {
	CBeforeSuite(func(ctx context.Context) {
		validateFlags()
		testLogger = logger.AddWriter(logger.NewLogger(*logLevel), GinkgoWriter)

		shoot := prepareShoot()
		shootGardenerTest, err = NewShootGardenerTest(*gardenerKubeconfigPath, shoot, testLogger)
		Expect(err).To(BeNil())
		if len(*workersConfig) == 0 {
			err := shootGardenerTest.SetupShootWorker(workerZone)
			Expect(err).To(BeNil())
		}
		Expect(len(shootGardenerTest.Shoot.Spec.Provider.Workers)).Should(BeNumerically("==", 1))

		// override default worker settings with flags
		if shootMachineType != nil && len(*shootMachineType) > 0 {
			shootGardenerTest.Shoot.Spec.Provider.Workers[0].Machine.Type = *shootMachineType
		}

		if shootMachineImageName != nil && len(*shootMachineImageName) > 0 {
			shootGardenerTest.Shoot.Spec.Provider.Workers[0].Machine.Image.Name = *shootMachineImageName
		}

		if shootMachineImageVersion != nil && len(*shootMachineImageVersion) > 0 {
			shootGardenerTest.Shoot.Spec.Provider.Workers[0].Machine.Image.Version = *shootMachineImageVersion
		}

		if StringSet(*seedName) {
			shootGardenerTest.Shoot.Spec.SeedName = seedName
		}

		gardenerTestOperation, err = NewGardenTestOperation(shootGardenerTest.GardenClient, testLogger)
		Expect(err).To(BeNil())
	}, InitializationTimeout)

	CIt("Create & Reconcile Shoot", func(ctx context.Context) {
		testLogger.Infof("Creating shoot %s in namespace %s", shootGardenerTest.Shoot.Name, *projectNamespace)
		if err := printShoot(shootGardenerTest.Shoot); err != nil {
			testLogger.Fatalf("Cannot decode shoot %s: %s", shootGardenerTest.Shoot.Name, err)
		}

		shootObject, err := shootGardenerTest.CreateShoot(ctx)
		if err != nil {
			if shootObject != nil {
				if err := gardenerTestOperation.AddShoot(ctx, shootObject); err != nil {
					testLogger.Errorf("Cannot add shoot %s to test operation: %s", shootGardenerTest.Shoot.Name, err.Error())
				}
				gardenerTestOperation.DumpState(ctx)
			}
			testLogger.Fatalf("Cannot create shoot %s: %s", shootGardenerTest.Shoot.Name, err.Error())
		}
		testLogger.Infof("Successfully created shoot %s", shootGardenerTest.Shoot.Name)

		if err := gardenerTestOperation.AddShoot(ctx, shootObject); err != nil {
			testLogger.Fatalf("Cannot add shoot %s to test operation: %s", shootGardenerTest.Shoot.Name, err.Error())
		}

		err = gardenerTestOperation.DownloadKubeconfig(ctx, gardenerTestOperation.SeedClient, gardenerTestOperation.ShootSeedNamespace(), "gardener", *shootKubeconfigPath)
		if err != nil {
			testLogger.Fatalf("Cannot download shoot gardenerKubeconfigPath: %s", err.Error())
		}
		err = gardenerTestOperation.DownloadKubeconfig(ctx, gardenerTestOperation.GardenClient, gardenerTestOperation.Seed.Spec.SecretRef.Namespace, gardenerTestOperation.Seed.Spec.SecretRef.Name, *seedKubeconfigPath)
		if err != nil {
			testLogger.Fatalf("Cannot download seed gardenerKubeconfigPath: %s", err.Error())
		}
		testLogger.Infof("Finished creating shoot %s", shootGardenerTest.Shoot.Name)
	}, CreateAndReconcileTimeout)
})

func prepareShoot() *gardencorev1beta1.Shoot {
	// if running in test machinery, test will be executed from root of the project
	if !FileExists(fmt.Sprintf(".%s", shootYamlPath)) {
		// locally, we need find the example shoot
		shootYamlPath = GetProjectRootPath() + shootYamlPath
		Expect(FileExists(shootYamlPath)).To(Equal(true))
	}
	// parse shoot yaml into shoot object and generate random test names for shoots
	_, shootObject, err := CreateShootTestArtifacts(shootYamlPath, testShootPrefix, projectNamespace, shootRegion, cloudProfile, secretBinding, shootProviderType, shootK8sVersion, externalDomain, true, true)
	Expect(err).To(BeNil())

	if testShootName != nil && len(*testShootName) > 0 {
		shootObject.Name = *testShootName
	}

	nginxIngress := &gardencorev1beta1.NginxIngress{Addon: gardencorev1beta1.Addon{Enabled: true}}
	if shootObject.Spec.Addons != nil {
		shootObject.Spec.Addons.NginxIngress = nginxIngress
	} else {
		shootObject.Spec.Addons = &gardencorev1beta1.Addons{NginxIngress: nginxIngress}
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

func printShoot(shoot *gardencorev1beta1.Shoot) error {
	d, err := yaml.Marshal(shoot)
	if err != nil {
		return err
	}
	fmt.Print(string(d))
	return nil
}
