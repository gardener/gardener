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

package shoot_creation

import (
	"context"
	"flag"
	"fmt"
	"github.com/gardener/gardener/test/framework"
	"path/filepath"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	"github.com/ghodss/yaml"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	// flags
	shootKubeconfigPath      = flag.String("shoot-kubecfg-path", "", "the path to where the Kubeconfig of the Shoot cluster will be downloaded to.")
	seedKubeconfigPath       = flag.String("seed-kubecfg-path", "", "the path to where the Kubeconfig of the Seed cluster will be downloaded to.")
	testShootName            = flag.String("shoot-name", "", "unique name to use for test shoots. Used by test-machinery.")
	testShootPrefix          = flag.String("prefix", "", "prefix for generated shoot name. Usually used locally to auto generate a unique name.")
	shootMachineImageName    = flag.String("machine-image-name", "", "the Machine Image Name of the test shoot. Defaults to first machine image in the CloudProfile.")
	shootMachineType         = flag.String("machine-type", "", "the Machine type of the first worker of the test shoot. Needs to match the machine types for that Provider available in the CloudProfile.")
	shootMachineImageVersion = flag.String("machine-image-version", "", "the Machine Image version of the first worker of the test shoot. Needs to be set when the MachineImageName is set.")
	cloudProfile             = flag.String("cloud-profile", "", "cloudProfile to use for the shoot.")
	seedName                 = flag.String("seed", "", "Name of the seed to use for the shoot.")
	shootRegion              = flag.String("region", "", "region to use for the shoot. Must be compatible with the infrastructureProvider.Zone.")
	secretBinding            = flag.String("secret-binding", "", "the secretBinding for the provider account of the shoot.")
	shootProviderType        = flag.String("provider-type", "", "the type of the cloud provider where the shoot is deployed to. e.g gcp, aws,azure,alicloud.")
	shootK8sVersion          = flag.String("k8s-version", "", "kubernetes version to use for the shoot.")
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
	shootYamlPath = flag.String("shoot-template-path", "default-shoot.yaml", "Specify the path to the shoot template that should be used to create the shoot")
)

const (
	CreateAndReconcileTimeout = 2 * time.Hour
)

func init() {
	framework.RegisterGardenerFrameworkFlags(nil)
}

func validateFlags() {

	if !framework.StringSet(*shootProviderType) {
		Fail("you need to specify provider type of the shoot")
	}

	if framework.StringSet(*shootMachineImageName) && !framework.StringSet(*shootMachineImageVersion) {
		Fail("shootMachineImageVersion has to be defined if shootMachineImageName is set")
	}

	if framework.StringSet(*shootMachineImageVersion) && !framework.StringSet(*shootMachineImageName) {
		Fail("shootMachineImageName has to be defined if shootMachineImageVersion is set")
	}

	if !framework.StringSet(*infrastructureProviderConfig) {
		Fail(fmt.Sprintf("you need to specify the filepath to the infrastructureProviderConfig for the provider '%s'", *shootProviderType))
	}

	if !framework.FileExists(*infrastructureProviderConfig) {
		Fail("path to the infrastructureProviderConfig of the Shoot is invalid")
	}

	if framework.StringSet(*controlPlaneProviderConfig) {
		if !framework.FileExists(*controlPlaneProviderConfig) {
			Fail("path to the controlPlaneProviderConfig of the Shoot is invalid")
		}
	}

	if framework.StringSet(*networkingProviderConfig) {
		if !framework.FileExists(*networkingProviderConfig) {
			Fail("path to the networkingProviderConfig of the Shoot is invalid")
		}
	}

	if framework.StringSet(*workersConfig) {
		if !framework.FileExists(*workersConfig) {
			Fail("path to the worker config of the Shoot is invalid")
		}
	}
}

var _ = Describe("Shoot Creation testing", func() {

	f := framework.NewGardenerFramework(&framework.GardenerConfig{
		CommonConfig: &framework.CommonConfig{
			ResourceDir: "../../framework/resources",
		},
	})

	framework.CIt("Create and Reconcile Shoot", func(ctx context.Context) {
		validateFlags()

		shoot := prepareShoot(ctx, f)

		// override default worker settings with flags
		if shootMachineType != nil && len(*shootMachineType) > 0 {
			for i := range shoot.Spec.Provider.Workers {
				shoot.Spec.Provider.Workers[i].Machine.Type = *shootMachineType
			}
		}

		if shootMachineImageName != nil && len(*shootMachineImageName) > 0 {
			for i := range shoot.Spec.Provider.Workers {
				shoot.Spec.Provider.Workers[i].Machine.Image.Name = *shootMachineImageName
			}
		}

		if shootMachineImageVersion != nil && len(*shootMachineImageVersion) > 0 {
			for i := range shoot.Spec.Provider.Workers {
				shoot.Spec.Provider.Workers[i].Machine.Image.Version = *shootMachineImageVersion
			}
		}

		if framework.StringSet(*seedName) {
			shoot.Spec.SeedName = seedName
		}

		f.Logger.Infof("Creating shoot %s in namespace %s", shoot.GetName(), f.ProjectNamespace)
		if err := printShoot(shoot); err != nil {
			f.Logger.Fatalf("Cannot decode shoot %s: %s", shoot.GetName(), err)
		}

		err := f.CreateShoot(ctx, shoot)
		if err != nil {
			if shoot != nil {
				shootFramework, err := f.NewShootFramework(shoot)
				framework.ExpectNoError(err)
				shootFramework.DumpState(ctx)
			}
			f.Logger.Fatalf("Cannot create shoot %s: %s", shoot.GetName(), err.Error())
		}
		f.Logger.Infof("Successfully created shoot %s", shoot.GetName())
		shootFramework, err := f.NewShootFramework(shoot)
		framework.ExpectNoError(err)

		if err := framework.DownloadKubeconfig(ctx, shootFramework.GardenClient, f.ProjectNamespace, shootFramework.ShootKubeconfigSecretName(), *shootKubeconfigPath); err != nil {
			f.Logger.Fatalf("Cannot download shoot kubeconfig: %s", err.Error())
		}

		if err := framework.DownloadKubeconfig(ctx, shootFramework.GardenClient, shootFramework.Seed.Spec.SecretRef.Namespace, shootFramework.Seed.Spec.SecretRef.Name, *seedKubeconfigPath); err != nil {
			f.Logger.Fatalf("Cannot download seed kubeconfig: %s", err.Error())
		}

		f.Logger.Infof("Finished creating shoot %s", shoot.GetName())
	}, CreateAndReconcileTimeout)
})

func prepareShoot(ctx context.Context, f *framework.GardenerFramework) *gardencorev1beta1.Shoot {
	// if running in test machinery, test will be executed from root of the project
	if !framework.FileExists(fmt.Sprintf(".%s", *shootYamlPath)) {
		// locally, we need find the example shoot
		*shootYamlPath = filepath.Join(f.TemplatesDir, *shootYamlPath)
		Expect(framework.FileExists(*shootYamlPath)).To(Equal(true), "shoot template should exist")
	}

	// parse shoot yaml into shoot object and generate random test names for shoots
	_, shootObject, err := framework.CreateShootTestArtifacts(*shootYamlPath, testShootPrefix, &f.ProjectNamespace, shootRegion, cloudProfile, secretBinding, shootProviderType, shootK8sVersion, externalDomain, true, true)
	Expect(err).ToNot(HaveOccurred())

	if testShootName != nil && len(*testShootName) > 0 {
		shootObject.Name = *testShootName
	}

	cloudprofile, err := f.GetCloudProfile(ctx, shootObject.Spec.CloudProfileName)
	Expect(err).ToNot(HaveOccurred())

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
	err = framework.SetProviderConfigsFromFilepath(shootObject, infrastructureProviderConfig, controlPlaneProviderConfig, networkingProviderConfig, workersConfig)
	Expect(err).To(BeNil())

	if len(*workersConfig) == 0 {
		err := framework.SetupShootWorker(shootObject, cloudprofile, workerZone)
		Expect(err).To(BeNil())
		Expect(len(shootObject.Spec.Provider.Workers)).Should(BeNumerically("==", 1))
	}

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
