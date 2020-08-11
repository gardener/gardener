// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"strconv"
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/onsi/ginkgo"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var seedRegistrationConfig *SeedRegistrationConfig

const (
	kubeconfigString = "kubeconfig"
)

// SeedRegistrationFramework represents the seed test framework that includes
// test functions that can be executed on a specific seed
type SeedRegistrationFramework struct {
	*GardenerFramework
	TestDescription
	Config *SeedRegistrationConfig

	SeedClient kubernetes.Interface
	Seed       *gardencorev1beta1.Seed
	Secret     *corev1.Secret //the secret that will be created and used for seed.Spec.SecretRef
}

// SeedRegistrationConfig is the configuration for a seed registration framework that will be filled with user provided data
type SeedRegistrationConfig struct {
	GardenerConfig         *GardenerConfig
	SeedName               string
	SecretBinding          string
	BackupSecretProvider   string
	ShootedSeedName        string
	ShootedSeedNamespace   string
	ProviderRegion         string
	ProviderType           string
	NetworkingPods         string
	NetworkingServices     string
	NetworkingNodes        string
	NetworkingBlockCIDRs   string
	DeployGardenletOnShoot string
}

// NewSeedRegistrationFramework creates a new simple SeedRegistrationFramework
func NewSeedRegistrationFramework(cfg *SeedRegistrationConfig) *SeedRegistrationFramework {
	var gardenerConfig *GardenerConfig
	if cfg != nil {
		gardenerConfig = cfg.GardenerConfig
	}

	f := &SeedRegistrationFramework{
		GardenerFramework: NewGardenerFrameworkFromConfig(gardenerConfig),
		TestDescription:   NewTestDescription("SEED"),
		Config:            cfg,
	}

	CBeforeEach(func(ctx context.Context) {
		f.CommonFramework.BeforeEach()
		f.GardenerFramework.BeforeEach()
		f.BeforeEach(ctx)
	}, 8*time.Minute)
	CAfterEach(f.AfterEach, 10*time.Minute)
	return f
}

// NewSeedRegistrationFrameworkFromConfig creates a new SeedRegistrationFramework from a seed configuration without registering ginkgo
func NewSeedRegistrationFrameworkFromConfig(cfg *SeedRegistrationConfig) (*SeedRegistrationFramework, error) {
	var gardenerConfig *GardenerConfig
	if cfg != nil {
		gardenerConfig = cfg.GardenerConfig
	}
	f := &SeedRegistrationFramework{
		GardenerFramework: NewGardenerFrameworkFromConfig(gardenerConfig),
		TestDescription:   NewTestDescription("SEED"),
		Config:            cfg,
	}
	return f, nil
}

// BeforeEach should be called in ginkgo's BeforeEach.
// It sets up the seed registration framework.
func (f *SeedRegistrationFramework) BeforeEach(ctx context.Context) {
	f.Config = mergeSeedConfig(f.Config, seedRegistrationConfig)
	validateSeedConfig(f.Config)
}

// AfterEach should be called in ginkgo's AfterEach.
// Cleans up resources and dumps the seed state if the test failed
func (f *SeedRegistrationFramework) AfterEach(ctx context.Context) {
	if ginkgo.CurrentGinkgoTestDescription().Failed {
		f.DumpState(ctx)
	}
}

func validateSeedConfig(cfg *SeedRegistrationConfig) {
	if cfg == nil {
		ginkgo.Fail("no seed registration framework configuration provided")
	}
	if !StringSet(cfg.SeedName) {
		ginkgo.Fail("You should specify a name for the new Seed")
	}
	if !StringSet(cfg.SecretBinding) {
		ginkgo.Fail("You should specify a secret binding for the secret that will be created for this seed")
	}
	if !StringSet(cfg.ShootedSeedName) || !StringSet(cfg.ShootedSeedNamespace) {
		ginkgo.Fail("You should specify Shooted Seed name and namespace to test against")
	}
	if !StringSet(cfg.ProviderRegion) || !StringSet(cfg.ProviderType) {
		ginkgo.Fail("You should specify a Provider and Provider type")
	}
	if !StringSet(cfg.NetworkingPods) || !StringSet(cfg.NetworkingServices) || !StringSet(cfg.NetworkingNodes) {
		ginkgo.Fail("Networking settings are missing or incomplete")
	}
	if StringSet(cfg.DeployGardenletOnShoot) {
		if _, err := strconv.ParseBool(cfg.DeployGardenletOnShoot); err != nil {
			ginkgo.Fail("-deploy-gardenlet is not a boolean value")
		}
	} else {
		cfg.DeployGardenletOnShoot = "true"
	}
}

func mergeSeedConfig(base, overwrite *SeedRegistrationConfig) *SeedRegistrationConfig {
	if base == nil {
		return overwrite
	}
	if overwrite == nil {
		return base
	}
	if overwrite.GardenerConfig != nil {
		base.GardenerConfig = overwrite.GardenerConfig
	}
	if StringSet(overwrite.SeedName) {
		base.SeedName = overwrite.SeedName
	}
	if StringSet(overwrite.SecretBinding) {
		base.SecretBinding = overwrite.SecretBinding
	}
	if StringSet(overwrite.BackupSecretProvider) {
		base.BackupSecretProvider = overwrite.BackupSecretProvider
	}
	if StringSet(overwrite.ShootedSeedName) {
		base.ShootedSeedName = overwrite.ShootedSeedName
	}
	if StringSet(overwrite.ShootedSeedNamespace) {
		base.ShootedSeedNamespace = overwrite.ShootedSeedNamespace
	}
	if StringSet(overwrite.ProviderRegion) {
		base.ProviderRegion = overwrite.ProviderRegion
	}
	if StringSet(overwrite.ProviderType) {
		base.ProviderType = overwrite.ProviderType
	}
	if StringSet(overwrite.NetworkingBlockCIDRs) {
		base.NetworkingBlockCIDRs = overwrite.NetworkingBlockCIDRs
	}
	if StringSet(overwrite.NetworkingPods) {
		base.NetworkingPods = overwrite.NetworkingPods
	}
	if StringSet(overwrite.NetworkingServices) {
		base.NetworkingServices = overwrite.NetworkingServices
	}
	if StringSet(overwrite.NetworkingNodes) {
		base.NetworkingNodes = overwrite.NetworkingNodes
	}
	if StringSet(overwrite.DeployGardenletOnShoot) {
		base.DeployGardenletOnShoot = overwrite.DeployGardenletOnShoot
	}
	return base
}

// RegisterSeedRegistrationFrameworkFlags adds all flags that are needed to configure a seed registration framework to the provided flagset.
func RegisterSeedRegistrationFrameworkFlags() *SeedRegistrationConfig {
	_ = RegisterGardenerFrameworkFlags()

	newCfg := &SeedRegistrationConfig{}

	flag.StringVar(&newCfg.SeedName, "seed-name", "", "Name of the seed")
	flag.StringVar(&newCfg.SecretBinding, "secret-binding", "", "Secret binding parameter")
	flag.StringVar(&newCfg.BackupSecretProvider, "backup-secret-provider", "", "Provider of the backup secret reference")
	// Shooted seed reference
	flag.StringVar(&newCfg.ShootedSeedName, "shoot-name", "", "Name of the shoot")
	flag.StringVar(&newCfg.ShootedSeedNamespace, "shoot-namespace", "", "Namespace of the shoot")
	// Provider
	flag.StringVar(&newCfg.ProviderType, "provider-type", "", "Provider type e.g. aws, azure, gcp")
	flag.StringVar(&newCfg.ProviderRegion, "provider-region", "", "Provider region e.g. eu-west-1")
	// Network
	flag.StringVar(&newCfg.NetworkingBlockCIDRs, "networking-blockcidrs", "", "Comma-separated values for seed.Spec.Networks.BlockCIDRs.")
	flag.StringVar(&newCfg.NetworkingPods, "networking-pods", "", "Value for Seed.Spec.Networks.Pods")
	flag.StringVar(&newCfg.NetworkingServices, "networking-services", "", "Value for Seed.Spec.Networks.Services")
	flag.StringVar(&newCfg.NetworkingNodes, "networking-nodes", "", "Value for Seed.Spec.Networks.Nodes")
	flag.StringVar(&newCfg.DeployGardenletOnShoot, "deploy-gardenlet", "", "Specify if a gardanlet instance should be installed on the shoot. Default = true")

	seedRegistrationConfig = newCfg

	return seedRegistrationConfig
}

// AddSeed function tries to get an existing seed using the configured GardenClient
// and the given seed parameter and adds it to the SeedRegistrationFramework
func (f *SeedRegistrationFramework) AddSeed(ctx context.Context, seed *gardencorev1beta1.Seed) error {
	if f.GardenClient == nil {
		return errors.New("no gardener client is defined")
	}
	if err := f.GardenClient.DirectClient().Get(ctx, client.ObjectKey{Name: seed.Name}, seed); err != nil {
		return err
	}
	f.Seed = seed
	return nil
}

// RegisterSeed registers a new seed and its secret reference (seed.Spec.SecretRef)
// according to the configuration provided for the SeedRegistrationFramework
func (f *SeedRegistrationFramework) RegisterSeed(ctx context.Context) (err error) {
	if f.GardenClient == nil {
		return errors.New("no gardener client is defined")
	}

	refShoot := &gardencorev1beta1.Shoot{}
	if err = f.GardenClient.DirectClient().Get(ctx, kutil.Key(f.Config.ShootedSeedNamespace, f.Config.ShootedSeedName), refShoot); err != nil {
		return err
	}

	if deploy, _ := strconv.ParseBool(f.Config.DeployGardenletOnShoot); deploy {
		return f.annotateShootToBeUsedAsSeed(ctx, refShoot)
	}

	secretBinding := &gardencorev1beta1.SecretBinding{}
	if err = f.GardenClient.DirectClient().Get(ctx, kutil.Key(f.Config.ShootedSeedNamespace, f.Config.SecretBinding), secretBinding); err != nil {
		return err
	}

	f.Seed = f.buildSeedObject(secretBinding, refShoot)

	if err = f.buildSeedSecret(ctx, secretBinding); err != nil {
		f.Logger.Errorf("Cannot build seed secret %s", err)
		return err
	}
	f.Logger.Infof("Creating seed secret %s in namespace %s", f.Secret.GetName(), f.Secret.GetNamespace())
	// Apply secret to the cluster
	if err = f.GardenClient.DirectClient().Create(ctx, f.Secret); err != nil {
		f.Logger.Errorf("Cannot create seed secret %s: %s", f.Secret.GetName(), err)
		return err
	}
	f.Logger.Infof("Secret resource %s was created!", f.Secret.Name)

	f.Logger.Infof("Registering seed %s", f.Seed.GetName())
	if err := PrettyPrintObject(f.Seed); err != nil {
		f.Logger.Errorf("Cannot decode seed %s: %s", f.Seed.GetName(), err)
		return err
	}
	return f.GardenerFramework.CreateSeed(ctx, f.Seed)
}

func (f *SeedRegistrationFramework) annotateShootToBeUsedAsSeed(ctx context.Context, refShoot *gardencorev1beta1.Shoot) (err error) {
	annotations := map[string]string{
		v1beta1constants.AnnotationShootUseAsSeed: "true,protected,invisible,with-secret-ref",
	}
	f.Logger.Infof("Annotating shoot with: %v", annotations)
	if err = f.GardenerFramework.AnnotateShoot(ctx, refShoot, annotations); err != nil {
		f.Logger.Errorf("Unable to annotate shoot due to: %s", err.Error())
		return err
	}
	return f.GardenerFramework.UpdateShoot(ctx, refShoot, func(shoot *gardencorev1beta1.Shoot) error {
		vpa := &gardencorev1beta1.VerticalPodAutoscaler{
			Enabled: true,
		}
		shoot.Spec.Kubernetes.VerticalPodAutoscaler = vpa

		return nil
	})
}

func (f *SeedRegistrationFramework) buildSeedObject(secretBinding *gardencorev1beta1.SecretBinding, refShoot *gardencorev1beta1.Shoot) *gardencorev1beta1.Seed {
	// TODO: Enable .spec.volume.minimumVolumeSize when implementing support for all providers
	seed := &gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			Name: f.Config.SeedName,
			Labels: map[string]string{
				v1beta1constants.GardenRole: v1beta1constants.GardenRoleSeed,
			},
		},
		Spec: gardencorev1beta1.SeedSpec{
			DNS: gardencorev1beta1.SeedDNS{
				IngressDomain: fmt.Sprintf("%s.%s", common.IngressPrefix, *refShoot.Spec.DNS.Domain),
			},
			SecretRef: &corev1.SecretReference{
				Namespace: f.Config.ShootedSeedNamespace,
				Name:      fmt.Sprintf("seed-%s", f.Config.SeedName),
			},
			Provider: gardencorev1beta1.SeedProvider{
				Region: f.Config.ProviderRegion,
				Type:   f.Config.ProviderType,
			},
			Networks: gardencorev1beta1.SeedNetworks{
				BlockCIDRs: strings.Split(f.Config.NetworkingBlockCIDRs, ","),
				Nodes:      &f.Config.NetworkingNodes,
				Pods:       f.Config.NetworkingPods,
				Services:   f.Config.NetworkingServices,
			},
			Settings: &gardencorev1beta1.SeedSettings{
				Scheduling: &gardencorev1beta1.SeedSettingScheduling{
					Visible: false,
				},
			},
		},
	}

	if StringSet(f.Config.BackupSecretProvider) {
		seed.Spec.Backup = &gardencorev1beta1.SeedBackup{
			SecretRef: corev1.SecretReference{
				Name:      secretBinding.SecretRef.Name,
				Namespace: secretBinding.SecretRef.Namespace,
			},
			Provider: f.Config.BackupSecretProvider,
		}
	}
	return seed
}

func (f *SeedRegistrationFramework) buildSeedSecret(ctx context.Context, secretBinding *gardencorev1beta1.SecretBinding) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: f.Config.ShootedSeedNamespace,
			Name:      fmt.Sprintf("seed-%s", f.Config.SeedName),
		},
	}

	kubeconfigSecret := &corev1.Secret{}
	if err := f.GardenClient.DirectClient().Get(ctx, kutil.Key(f.Config.ShootedSeedNamespace, (f.Config.ShootedSeedName+".kubeconfig")), kubeconfigSecret); err != nil {
		return err
	}
	kubecfgData := kubeconfigSecret.Data["kubeconfig"]

	referenceSecret := &corev1.Secret{}
	if err := f.GardenClient.DirectClient().Get(ctx, kutil.Key(secretBinding.SecretRef.Namespace, secretBinding.SecretRef.Name), referenceSecret); err != nil {
		return err
	}
	secret.Data = referenceSecret.Data
	secret.Data[kubeconfigString] = kubecfgData

	f.Secret = secret

	return nil
}
