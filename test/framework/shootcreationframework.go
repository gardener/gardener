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

package framework

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	"github.com/onsi/ginkgo"
)

var shootCreationCfg *ShootCreationConfig

// ShootCreationConfig is the configuration for a shoot creation framework
type ShootCreationConfig struct {
	GardenerConfig *GardenerConfig

	shootKubeconfigPath          string
	seedKubeconfigPath           string
	testShootName                string
	testShootPrefix              string
	shootMachineImageName        string
	shootMachineType             string
	shootMachineImageVersion     string
	cloudProfile                 string
	seedName                     string
	shootRegion                  string
	secretBinding                string
	shootProviderType            string
	shootK8sVersion              string
	externalDomain               string
	workerZone                   string
	networkingPods               string
	networkingServices           string
	networkingNodes              string
	startHibernatedFlag          string
	startHibernated              bool
	infrastructureProviderConfig string
	controlPlaneProviderConfig   string
	networkingProviderConfig     string
	workersConfig                string
	shootYamlPath                string
}

// ShootCreationFramework represents the shoot test framework that includes
// test functions that can be executed ona specific shoot
type ShootCreationFramework struct {
	*GardenerFramework
	TestDescription
	Config *ShootCreationConfig

	Shoot *gardencorev1beta1.Shoot

	shootFramework *ShootFramework
}

// NewShootCreationFramework creates a new simple Shoot creation framework
func NewShootCreationFramework(cfg *ShootCreationConfig) *ShootCreationFramework {
	var gardenerConfig *GardenerConfig
	if cfg != nil {
		gardenerConfig = cfg.GardenerConfig
	}

	f := &ShootCreationFramework{
		GardenerFramework: NewGardenerFramework(gardenerConfig),
		TestDescription:   NewTestDescription("SHOOTCREATION"),
		Config:            cfg,
	}

	ginkgo.BeforeEach(func() {
		f.GardenerFramework.BeforeEach()
		f.BeforeEach()
	})
	CAfterEach(f.AfterEach, 10*time.Minute)
	return f
}

// BeforeEach should be called in ginkgo's BeforeEach.
// It sets up the shoot creation framework.
func (f *ShootCreationFramework) BeforeEach() {
	f.Config = mergeShootCreationConfig(f.Config, shootCreationCfg)
	validateShootCreationConfig(f.Config)
}

// AfterEach should be called in ginkgo's AfterEach.
// Cleans up resources and dumps the shoot state if the test failed
func (f *ShootCreationFramework) AfterEach(ctx context.Context) {
	if ginkgo.CurrentGinkgoTestDescription().Failed {
		f.DumpState(ctx)
	}
}

func validateShootCreationConfig(cfg *ShootCreationConfig) {
	if cfg == nil {
		ginkgo.Fail("no shoot creation framework configuration provided")
	}

	if !StringSet(cfg.shootProviderType) {
		ginkgo.Fail("you need to specify provider type of the shoot")
	}

	if StringSet(cfg.shootMachineImageName) && !StringSet(cfg.shootMachineImageVersion) {
		ginkgo.Fail("shootMachineImageVersion has to be defined if shootMachineImageName is set")
	}

	if StringSet(cfg.shootMachineImageVersion) && !StringSet(cfg.shootMachineImageName) {
		ginkgo.Fail("shootMachineImageName has to be defined if shootMachineImageVersion is set")
	}

	if StringSet(cfg.startHibernatedFlag) {
		parsedBool, err := strconv.ParseBool(cfg.startHibernatedFlag)
		if err != nil {
			ginkgo.Fail("startHibernated is not a boolean value")
		}
		cfg.startHibernated = parsedBool
	}

	if !StringSet(cfg.infrastructureProviderConfig) {
		ginkgo.Fail(fmt.Sprintf("you need to specify the filepath to the infrastructureProviderConfig for the provider '%s'", cfg.shootProviderType))
	}

	if !FileExists(cfg.infrastructureProviderConfig) {
		ginkgo.Fail("path to the infrastructureProviderConfig of the Shoot is invalid")
	}

	if StringSet(cfg.controlPlaneProviderConfig) {
		if !FileExists(cfg.controlPlaneProviderConfig) {
			ginkgo.Fail("path to the controlPlaneProviderConfig of the Shoot is invalid")
		}
	}

	if StringSet(cfg.networkingProviderConfig) {
		if !FileExists(cfg.networkingProviderConfig) {
			ginkgo.Fail("path to the networkingProviderConfig of the Shoot is invalid")
		}
	}

	if StringSet(cfg.workersConfig) {
		if !FileExists(cfg.workersConfig) {
			ginkgo.Fail("path to the worker config of the Shoot is invalid")
		}
	}
}

func mergeShootCreationConfig(base, overwrite *ShootCreationConfig) *ShootCreationConfig {
	if base == nil {
		return overwrite
	}
	if overwrite == nil {
		return base
	}

	if overwrite.GardenerConfig != nil {
		base.GardenerConfig = mergeGardenerConfig(base.GardenerConfig, overwrite.GardenerConfig)
	}

	if StringSet(overwrite.shootKubeconfigPath) {
		base.shootKubeconfigPath = overwrite.shootKubeconfigPath
	}

	if StringSet(overwrite.seedKubeconfigPath) {
		base.seedKubeconfigPath = overwrite.seedKubeconfigPath
	}

	if StringSet(overwrite.testShootName) {
		base.testShootName = overwrite.testShootName
	}

	if StringSet(overwrite.testShootPrefix) {
		base.testShootPrefix = overwrite.testShootPrefix
	}

	if StringSet(overwrite.shootMachineImageName) {
		base.shootMachineImageName = overwrite.shootMachineImageName
	}

	if StringSet(overwrite.shootMachineType) {
		base.shootMachineType = overwrite.shootMachineType
	}

	if StringSet(overwrite.shootMachineImageVersion) {
		base.shootMachineImageVersion = overwrite.shootMachineImageVersion
	}

	if StringSet(overwrite.cloudProfile) {
		base.cloudProfile = overwrite.cloudProfile
	}

	if StringSet(overwrite.seedName) {
		base.seedName = overwrite.seedName
	}

	if StringSet(overwrite.shootRegion) {
		base.shootRegion = overwrite.shootRegion
	}

	if StringSet(overwrite.secretBinding) {
		base.secretBinding = overwrite.secretBinding
	}

	if StringSet(overwrite.shootProviderType) {
		base.shootProviderType = overwrite.shootProviderType
	}

	if StringSet(overwrite.shootK8sVersion) {
		base.shootK8sVersion = overwrite.shootK8sVersion
	}

	if StringSet(overwrite.externalDomain) {
		base.externalDomain = overwrite.externalDomain
	}

	if StringSet(overwrite.workerZone) {
		base.workerZone = overwrite.workerZone
	}

	if StringSet(overwrite.networkingPods) {
		base.networkingPods = overwrite.networkingPods
	}

	if StringSet(overwrite.networkingServices) {
		base.networkingServices = overwrite.networkingServices
	}

	if StringSet(overwrite.networkingNodes) {
		base.networkingNodes = overwrite.networkingNodes
	}

	if StringSet(overwrite.startHibernatedFlag) {
		base.startHibernatedFlag = overwrite.startHibernatedFlag
	}

	if overwrite.startHibernated {
		base.startHibernated = overwrite.startHibernated
	}

	if StringSet(overwrite.infrastructureProviderConfig) {
		base.infrastructureProviderConfig = overwrite.infrastructureProviderConfig
	}

	if StringSet(overwrite.controlPlaneProviderConfig) {
		base.controlPlaneProviderConfig = overwrite.controlPlaneProviderConfig
	}

	if StringSet(overwrite.networkingProviderConfig) {
		base.networkingProviderConfig = overwrite.networkingProviderConfig
	}

	if StringSet(overwrite.workersConfig) {
		base.workersConfig = overwrite.workersConfig
	}

	if StringSet(overwrite.shootYamlPath) {
		base.shootYamlPath = overwrite.shootYamlPath
	}

	return base
}

// RegisterShootCreationFrameworkFlags adds all flags that are needed to configure a shoot creation framework to the provided flagset.
func RegisterShootCreationFrameworkFlags() *ShootCreationConfig {
	_ = RegisterGardenerFrameworkFlags()

	newCfg := &ShootCreationConfig{}

	flag.StringVar(&newCfg.shootKubeconfigPath, "shoot-kubecfg-path", "", "the path to where the Kubeconfig of the Shoot cluster will be downloaded to.")
	flag.StringVar(&newCfg.seedKubeconfigPath, "seed-kubecfg-path", "", "the path to where the Kubeconfig of the Seed cluster will be downloaded to.")
	flag.StringVar(&newCfg.testShootName, "shoot-name", "", "unique name to use for test shoots. Used by test-machinery.")
	flag.StringVar(&newCfg.testShootPrefix, "prefix", "", "prefix for generated shoot name. Usually used locally to auto generate a unique name.")
	flag.StringVar(&newCfg.shootMachineImageName, "machine-image-name", "", "the Machine Image Name of the test shoot. Defaults to first machine image in the CloudProfile.")
	flag.StringVar(&newCfg.shootMachineType, "machine-type", "", "the Machine type of the first worker of the test shoot. Needs to match the machine types for that Provider available in the CloudProfile.")
	flag.StringVar(&newCfg.shootMachineImageVersion, "machine-image-version", "", "the Machine Image version of the first worker of the test shoot. Needs to be set when the MachineImageName is set.")
	flag.StringVar(&newCfg.cloudProfile, "cloud-profile", "", "cloudProfile to use for the shoot.")
	flag.StringVar(&newCfg.seedName, "seed", "", "Name of the seed to use for the shoot.")
	flag.StringVar(&newCfg.shootRegion, "region", "", "region to use for the shoot. Must be compatible with the infrastructureProvider.Zone.")
	flag.StringVar(&newCfg.secretBinding, "secret-binding", "", "the secretBinding for the provider account of the shoot.")
	flag.StringVar(&newCfg.shootProviderType, "provider-type", "", "the type of the cloud provider where the shoot is deployed to. e.g gcp, aws,azure,alicloud.")
	flag.StringVar(&newCfg.shootK8sVersion, "k8s-version", "", "kubernetes version to use for the shoot.")
	flag.StringVar(&newCfg.externalDomain, "external-domain", "", "external domain to use for the shoot. If not set, will use the default domain.")
	flag.StringVar(&newCfg.workerZone, "worker-zone", "", "zone to use for every worker of the shoot.")
	flag.StringVar(&newCfg.networkingPods, "networking-pods", "", "the spec.networking.pods to use for this shoot. Optional.")
	flag.StringVar(&newCfg.networkingServices, "networking-services", "", "the spec.networking.services to use for this shoot. Optional.")
	flag.StringVar(&newCfg.networkingNodes, "networking-nodes", "", "the spec.networking.nodes to use for this shoot. Optional.")
	flag.StringVar(&newCfg.startHibernatedFlag, "start-hibernated", "", "the spec.hibernation.enabled to use for this shoot. Optional.")
	newCfg.startHibernated = false

	// ProviderConfigs flags
	flag.StringVar(&newCfg.infrastructureProviderConfig, "infrastructure-provider-config-filepath", "", "filepath to the provider specific infrastructure config.")
	flag.StringVar(&newCfg.controlPlaneProviderConfig, "controlplane-provider-config-filepath", "", "filepath to the provider specific infrastructure config.")
	flag.StringVar(&newCfg.networkingProviderConfig, "networking-provider-config-filepath", "", "filepath to the provider specific infrastructure config.")
	flag.StringVar(&newCfg.workersConfig, "workers-config-filepath", "", "filepath to the worker config.")

	// other
	flag.StringVar(&newCfg.shootYamlPath, "shoot-template-path", "default-shoot.yaml", "Specify the path to the shoot template that should be used to create the shoot")

	shootCreationCfg = newCfg
	return shootCreationCfg
}

func (f *ShootCreationFramework) CreateShoot(ctx context.Context, initializeShootWithFlags, waitUntilShootIsReconciled bool) (*gardencorev1beta1.Shoot, error) {
	if initializeShootWithFlags {
		if err := f.InitializeShootWithFlags(ctx); err != nil {
			return nil, err
		}
	}

	f.Logger.Infof("Creating shoot %s in namespace %s", f.Shoot.GetName(), f.ProjectNamespace)
	if err := PrettyPrintObject(f.Shoot); err != nil {
		f.Logger.Fatalf("Cannot decode shoot %s: %s", f.Shoot.GetName(), err)
		return nil, err
	}

	if !waitUntilShootIsReconciled {
		return f.GardenerFramework.createShootResource(ctx, f.Shoot)
	}

	if err := f.GardenerFramework.CreateShoot(ctx, f.Shoot); err != nil {
		f.Logger.Fatalf("Cannot create shoot %s: %s", f.Shoot.GetName(), err.Error())
		shootFramework, err2 := f.newShootFramework()
		if err2 != nil {
			f.Logger.Fatalf("Cannot dump shoot state %s: %s", f.Shoot.GetName(), err.Error())
		} else {
			shootFramework.DumpState(ctx)
		}
		return nil, err
	}

	f.Logger.Infof("Successfully created shoot %s", f.Shoot.GetName())
	shootFramework, err := f.newShootFramework()
	if err != nil {
		return nil, err
	}
	f.shootFramework = shootFramework

	if err := DownloadKubeconfig(ctx, shootFramework.GardenClient, f.ProjectNamespace, shootFramework.ShootKubeconfigSecretName(), f.Config.shootKubeconfigPath); err != nil {
		f.Logger.Fatalf("Cannot download shoot kubeconfig: %s", err.Error())
		return nil, err
	}

	if err := DownloadKubeconfig(ctx, shootFramework.GardenClient, shootFramework.Seed.Spec.SecretRef.Namespace, shootFramework.Seed.Spec.SecretRef.Name, f.Config.seedKubeconfigPath); err != nil {
		f.Logger.Fatalf("Cannot download seed kubeconfig: %s", err.Error())
		return nil, err
	}

	f.Logger.Infof("Finished creating shoot %s", f.Shoot.GetName())

	return f.Shoot, nil
}

func (f *ShootCreationFramework) InitializeShootWithFlags(ctx context.Context) error {
	// if running in test machinery, test will be executed from root of the project
	if !FileExists(fmt.Sprintf(".%s", f.Config.shootYamlPath)) {
		// locally, we need find the example shoot
		f.Config.shootYamlPath = filepath.Join(f.TemplatesDir, f.Config.shootYamlPath)
		if !FileExists(f.Config.shootYamlPath) {
			return fmt.Errorf("shoot template should exist")
		}
	}

	// parse shoot yaml into shoot object and generate random test names for shoots
	_, shootObject, err := CreateShootTestArtifacts(f.Config, f.ProjectNamespace, true, true)
	if err != nil {
		return err
	}
	f.Shoot = shootObject

	// set ProviderConfigs
	err = SetProviderConfigsFromFilepath(shootObject, f.Config.infrastructureProviderConfig, f.Config.controlPlaneProviderConfig, f.Config.networkingProviderConfig)
	if err != nil {
		return err
	}

	// set worker settings
	cloudProfile, err := f.GetCloudProfile(ctx, shootObject.Spec.CloudProfileName)
	if err != nil {
		return err
	}

	if err = setShootWorkerSettings(shootObject, f.Config, cloudProfile); err != nil {
		return err
	}

	return nil
}

// newShootFramework creates a new ShootFramework with the Shoot created by the ShootCreationFramework
func (f *ShootCreationFramework) newShootFramework() (*ShootFramework, error) {
	shootFramework, err := f.GardenerFramework.NewShootFramework(f.Shoot)
	if err != nil {
		return nil, err
	}

	return shootFramework, nil
}

// GetShootFramework returns a ShootFramework for the Shoot created by the ShootCreationFramework
func (f *ShootCreationFramework) GetShootFramework() *ShootFramework {
	return f.shootFramework
}
