// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	shootoperation "github.com/gardener/gardener/test/utils/shoots/operation"
)

var shootCreationCfg *ShootCreationConfig

// ShootCreationConfig is the configuration for a shoot creation framework
type ShootCreationConfig struct {
	GardenerConfig *GardenerConfig

	shootKubeconfigPath           string
	testShootName                 string
	testShootPrefix               string
	shootMachineImageName         string
	shootMachineType              string
	shootMachineImageVersion      string
	cloudProfileName              string
	cloudProfileKind              string
	seedName                      string
	shootRegion                   string
	secretBinding                 string
	credentialsBinding            string
	shootProviderType             string
	shootK8sVersion               string
	externalDomain                string
	workerZone                    string
	ipFamilies                    string
	networkingType                string
	networkingPods                string
	networkingServices            string
	networkingNodes               string
	startHibernatedFlag           string
	startHibernated               bool
	infrastructureProviderConfig  string
	controlPlaneProviderConfig    string
	networkingProviderConfig      string
	workersConfig                 string
	shootYamlPath                 string
	shootAnnotations              string
	controlPlaneFailureTolerance  string
	kubeApiserverMinAllowedCPU    string
	kubeApiserverMinAllowedMemory string
	etcdMinAllowedCPU             string
	etcdMinAllowedMemory          string
}

// ShootCreationFramework represents the shoot test framework that includes
// test functions that can be executed ona specific shoot
type ShootCreationFramework struct {
	*GardenerFramework
	TestDescription
	Config *ShootCreationConfig

	Shoot *gardencorev1beta1.Shoot

	// ShootFramework is initialized once the shoot has been created successfully
	ShootFramework *ShootFramework
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
	if f.Shoot == nil {
		f.Config = mergeShootCreationConfig(f.Config, shootCreationCfg)
		validateShootCreationConfig(f.Config)
	}
}

// AfterEach should be called in ginkgo's AfterEach.
// Cleans up resources and dumps the shoot state if the test failed
func (f *ShootCreationFramework) AfterEach(ctx context.Context) {
	if ginkgo.CurrentSpecReport().Failed() {
		f.DumpState(ctx)
	}
}

func validateShootCreationConfig(cfg *ShootCreationConfig) {
	if cfg == nil {
		ginkgo.Fail("no shoot creation framework configuration provided")
		return
	}

	if StringSet(cfg.shootAnnotations) {
		_, err := parseAnnotationCfg(cfg.shootAnnotations)
		if err != nil {
			ginkgo.Fail(fmt.Sprintf("annotations could not be parsed: %+v", err))
		}
	}

	if StringSet(cfg.credentialsBinding) && StringSet(cfg.secretBinding) {
		ginkgo.Fail("you cannot specify both credentialsBinding and secretBinding for the shoot, please use only one of them")
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

	if StringSet(cfg.infrastructureProviderConfig) {
		if !FileExists(cfg.infrastructureProviderConfig) {
			ginkgo.Fail(fmt.Sprintf("you need to specify the filepath to the infrastructureProviderConfig for the provider '%s'", cfg.shootProviderType))
		}
	}

	if StringSet(cfg.controlPlaneProviderConfig) {
		if !FileExists(cfg.controlPlaneProviderConfig) {
			ginkgo.Fail(fmt.Sprintf("path to the controlPlaneProviderConfig of the Shoot is invalid: %s", cfg.controlPlaneProviderConfig))
		}
	}

	if StringSet(cfg.networkingProviderConfig) {
		if !FileExists(cfg.networkingProviderConfig) {
			ginkgo.Fail(fmt.Sprintf("path to the networkingProviderConfig of the Shoot is invalid: %s", cfg.networkingProviderConfig))
		}
	}

	if StringSet(cfg.workersConfig) {
		if !FileExists(cfg.workersConfig) {
			ginkgo.Fail(fmt.Sprintf("path to the worker config of the Shoot is invalid: %s", cfg.workersConfig))
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

	if StringSet(overwrite.testShootName) {
		base.testShootName = overwrite.testShootName
	}

	if StringSet(overwrite.testShootPrefix) {
		base.testShootPrefix = overwrite.testShootPrefix
	}

	if StringSet(overwrite.shootAnnotations) {
		base.shootAnnotations = overwrite.shootAnnotations
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

	if StringSet(overwrite.cloudProfileName) {
		base.cloudProfileName = overwrite.cloudProfileName
	}

	if StringSet(overwrite.cloudProfileKind) {
		base.cloudProfileKind = overwrite.cloudProfileKind
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

	if StringSet(overwrite.credentialsBinding) {
		base.credentialsBinding = overwrite.credentialsBinding
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

	if StringSet(overwrite.ipFamilies) {
		base.ipFamilies = overwrite.ipFamilies
	}

	if StringSet(overwrite.networkingType) {
		base.networkingType = overwrite.networkingType
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

	if StringSet(overwrite.controlPlaneFailureTolerance) {
		base.controlPlaneFailureTolerance = overwrite.controlPlaneFailureTolerance
	}

	if StringSet(overwrite.kubeApiserverMinAllowedCPU) {
		base.kubeApiserverMinAllowedCPU = overwrite.kubeApiserverMinAllowedCPU
	}

	if StringSet(overwrite.kubeApiserverMinAllowedMemory) {
		base.kubeApiserverMinAllowedMemory = overwrite.kubeApiserverMinAllowedMemory
	}

	if StringSet(overwrite.etcdMinAllowedCPU) {
		base.etcdMinAllowedCPU = overwrite.etcdMinAllowedCPU
	}

	if StringSet(overwrite.etcdMinAllowedMemory) {
		base.etcdMinAllowedMemory = overwrite.etcdMinAllowedMemory
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

	flag.StringVar(&newCfg.shootKubeconfigPath, "shoot-kubecfg-path", "", "the path to where the Kubeconfig of the Shoot cluster will be downloaded to. The kubeconfig expires in 6 hours.")
	flag.StringVar(&newCfg.testShootName, "shoot-name", "", "unique name to use for test shoots. Used by test-machinery.")
	flag.StringVar(&newCfg.testShootPrefix, "prefix", "", "prefix for generated shoot name. Usually used locally to auto generate a unique name.")
	flag.StringVar(&newCfg.shootAnnotations, "annotations", "", "annotations to be added to the test shoot. Expected format is key1=val1,key2=val2 (similar to kubectl --selector).")
	flag.StringVar(&newCfg.shootMachineImageName, "machine-image-name", "", "the Machine Image Name of the test shoot. Defaults to first machine image in the CloudProfile.")
	flag.StringVar(&newCfg.shootMachineType, "machine-type", "", "the Machine type of the first worker of the test shoot. Needs to match the machine types for that Provider available in the CloudProfile.")
	flag.StringVar(&newCfg.shootMachineImageVersion, "machine-image-version", "", "the Machine Image version of the first worker of the test shoot. Needs to be set when the MachineImageName is set.")
	flag.StringVar(&newCfg.cloudProfileName, "cloud-profile-name", "", "cloudProfile name to use for the shoot.")
	flag.StringVar(&newCfg.cloudProfileKind, "cloud-profile-kind", v1beta1constants.CloudProfileReferenceKindCloudProfile, "cloudProfile kind to use for the shoot. Optional.")
	flag.StringVar(&newCfg.seedName, "seed", "", "Name of the seed to use for the shoot.")
	flag.StringVar(&newCfg.shootRegion, "region", "", "region to use for the shoot. Must be compatible with the infrastructureProvider.Zone.")
	flag.StringVar(&newCfg.secretBinding, "secret-binding", "", "the secretBinding for the provider account of the shoot.")
	flag.StringVar(&newCfg.credentialsBinding, "credentials-binding", "", "the credentialsBinding for the provider account of the shoot.")
	flag.StringVar(&newCfg.shootProviderType, "provider-type", "", "the type of the cloud provider where the shoot is deployed to. e.g gcp, aws,azure,alicloud.")
	flag.StringVar(&newCfg.shootK8sVersion, "k8s-version", "", "kubernetes version to use for the shoot.")
	flag.StringVar(&newCfg.externalDomain, "external-domain", "", "external domain to use for the shoot. If not set, will use the default domain.")
	flag.StringVar(&newCfg.workerZone, "worker-zone", "", "zone to use for every worker of the shoot.")
	flag.StringVar(&newCfg.ipFamilies, "ip-families", "", "the spec.networking.ipFamilies to use for this shoot. Optional. Defaults to an empty string resulting in IPv4. Use a comma separated list to provide multiple values, e.g. 'IPv6,IPv4'.")
	flag.StringVar(&newCfg.networkingType, "networking-type", "calico", "the spec.networking.type to use for this shoot. Optional. Defaults to calico.")
	flag.StringVar(&newCfg.networkingPods, "networking-pods", "", "the spec.networking.pods to use for this shoot. Optional.")
	flag.StringVar(&newCfg.networkingServices, "networking-services", "", "the spec.networking.services to use for this shoot. Optional.")
	flag.StringVar(&newCfg.networkingNodes, "networking-nodes", "", "the spec.networking.nodes to use for this shoot. Optional.")
	flag.StringVar(&newCfg.startHibernatedFlag, "start-hibernated", "", "the spec.hibernation.enabled to use for this shoot. Optional.")
	flag.StringVar(&newCfg.controlPlaneFailureTolerance, "control-plane-failure-tolerance", "", "the .spec.controlPlane.HighAvailability.FailureTolerance.FailureToleranceType to use for this shoot. Optional, defaults to no failure tolerance")
	flag.StringVar(&newCfg.kubeApiserverMinAllowedCPU, "kube-apiserver-min-allowed-cpu", "", "the .spec.kubernetes.kubeAPIServer.autoscaling.cpu to use for this shoot. Optional.")
	flag.StringVar(&newCfg.kubeApiserverMinAllowedMemory, "kube-apiserver-min-allowed-memory", "", "the .spec.kubernetes.kubeAPIServer.autoscaling.memory to use for this shoot. Optional.")
	flag.StringVar(&newCfg.etcdMinAllowedCPU, "etcd-min-allowed-cpu", "", "the .spec.kubernetes.etcd.{main|events}.autoscaling.cpu to use for this shoot. Optional.")
	flag.StringVar(&newCfg.etcdMinAllowedMemory, "etcd-min-allowed-memory", "", "the .spec.kubernetes.etcd.{main|events}.autoscaling.memory to use for this shoot. Optional.")

	if newCfg.networkingType == "" {
		newCfg.networkingType = "calico"
	}

	newCfg.startHibernated = false

	// ProviderConfigs flags
	flag.StringVar(&newCfg.infrastructureProviderConfig, "infrastructure-provider-config-filepath", "", "filepath to the provider specific infrastructure config.")
	flag.StringVar(&newCfg.controlPlaneProviderConfig, "controlplane-provider-config-filepath", "", "filepath to the control plane config.")
	flag.StringVar(&newCfg.networkingProviderConfig, "networking-provider-config-filepath", "", "filepath to the network provider config.")
	flag.StringVar(&newCfg.workersConfig, "workers-config-filepath", "", "filepath to the worker config.")

	// other
	flag.StringVar(&newCfg.shootYamlPath, "shoot-template-path", "default-shoot.yaml", "Specify the path to the shoot template that should be used to create the shoot")

	shootCreationCfg = newCfg
	return shootCreationCfg
}

// CreateShootAndWaitForCreation creates a shoot using this framework's configuration and waits for successful creation.
func (f *ShootCreationFramework) CreateShootAndWaitForCreation(ctx context.Context, initializeShootWithFlags bool) error {
	if initializeShootWithFlags {
		if err := f.InitializeShootWithFlags(ctx); err != nil {
			return err
		}
	} else {
		if f.Shoot.Namespace == "" {
			f.Shoot.Namespace = f.ProjectNamespace
		}
	}

	log := f.Logger.WithValues("shoot", client.ObjectKeyFromObject(f.Shoot))

	if f.GardenerFramework.Config.ExistingShootName != "" {
		shootKey := client.ObjectKey{Namespace: f.ProjectNamespace, Name: f.GardenerFramework.Config.ExistingShootName}
		if err := f.GardenClient.Client().Get(ctx, shootKey, f.Shoot); err != nil {
			return fmt.Errorf("failed to get existing shoot %q: %w", shootKey, err)
		}

		shootHealthy, msg := shootoperation.ReconciliationSuccessful(f.Shoot)
		if !shootHealthy {
			return fmt.Errorf("cannot use existing shoot %q for test because it is unhealthy: %s", shootKey, msg)
		}

		f.Logger.Info("Using existing shoot for test", "shoot", shootKey)
		if err := PrettyPrintObject(f.Shoot); err != nil {
			return err
		}
	} else {
		log.Info("Creating shoot")
		if err := PrettyPrintObject(f.Shoot); err != nil {
			return err
		}

		if err := f.CreateShoot(ctx, f.Shoot, true); err != nil {
			log.Error(err, "Failed creating shoot")

			dumpCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if shootFramework, err := f.NewShootFramework(dumpCtx, f.Shoot); err != nil {
				log.Error(err, "Failed dumping shoot state")
			} else {
				shootFramework.DumpState(dumpCtx)
			}
			return err
		}

		log.Info("Successfully created shoot")
	}

	shootFramework, err := f.NewShootFramework(ctx, f.Shoot)
	if err != nil {
		return err
	}
	f.ShootFramework = shootFramework
	f.Shoot = shootFramework.Shoot

	if f.Config.shootKubeconfigPath == "" {
		f.Logger.Info("Shoot kubeconfig path is not specified, skipping downloading the admin kubeconfig for the Shoot")
	} else {
		if err := DownloadAdminKubeconfigForShoot(ctx, shootFramework.GardenClient, shootFramework.Shoot, f.Config.shootKubeconfigPath); err != nil {
			return fmt.Errorf("failed downloading shoot kubeconfig: %w", err)
		}
	}

	log.Info("Finished creating shoot")
	return nil
}

// Verify asserts that the shoot creation was successful.
func (f *ShootCreationFramework) Verify() {
	var (
		expectedTechnicalID           = fmt.Sprintf("shoot--%s--%s", f.ShootFramework.Project.Name, f.Shoot.Name)
		expectedClusterIdentityPrefix = fmt.Sprintf("%s-%s", f.Shoot.Status.TechnicalID, f.Shoot.Status.UID)
	)

	// Shoot with failure tolerance 'zone' should only be scheduled on seed with at least 3 zones.
	if v1beta1helper.IsMultiZonalShootControlPlane(f.Shoot) {
		gomega.Expect(len(f.ShootFramework.Seed.Spec.Provider.Zones)).Should(gomega.BeNumerically(">=", 3))
	}

	gomega.Expect(f.Shoot.Status.Gardener.ID).NotTo(gomega.BeEmpty())
	gomega.Expect(f.Shoot.Status.Gardener.Name).NotTo(gomega.BeEmpty())
	gomega.Expect(f.Shoot.Status.Gardener.Version).NotTo(gomega.BeEmpty())
	gomega.Expect(f.Shoot.Status.LastErrors).To(gomega.BeEmpty())
	gomega.Expect(f.Shoot.Status.SeedName).NotTo(gomega.BeNil())
	gomega.Expect(*f.Shoot.Status.SeedName).NotTo(gomega.BeEmpty())
	gomega.Expect(f.Shoot.Status.TechnicalID).To(gomega.Equal(expectedTechnicalID))
	gomega.Expect(f.Shoot.Status.UID).NotTo(gomega.BeEmpty())
	gomega.Expect(f.Shoot.Status.ClusterIdentity).NotTo(gomega.BeNil())
	gomega.Expect(*f.Shoot.Status.ClusterIdentity).To(gomega.HavePrefix(expectedClusterIdentityPrefix))
}

// InitializeShootWithFlags initializes a shoot to be created by this framework.
func (f *ShootCreationFramework) InitializeShootWithFlags(ctx context.Context) error {
	// if running in test machinery, test will be executed from root of the project
	if !FileExists(fmt.Sprintf(".%s", f.Config.shootYamlPath)) {
		path := f.Config.shootYamlPath
		if !filepath.IsAbs(f.Config.shootYamlPath) {
			// locally, we need find the example shoot
			path = filepath.Join(f.TemplatesDir, f.Config.shootYamlPath)
		}
		f.Config.shootYamlPath = path
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
	cloudProfile, err := f.GetCloudProfile(ctx, shootObject.Spec.CloudProfile, shootObject.Namespace, shootObject.Spec.CloudProfileName)
	if err != nil {
		return err
	}

	return setShootWorkerSettings(shootObject, f.Config, cloudProfile)
}
