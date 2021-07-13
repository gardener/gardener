// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"errors"
	"flag"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/onsi/ginkgo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var managedSeedConfig *ManagedSeedConfig

// ManagedSeedFramework is a test framework for testing managed seed creation and deletion.
type ManagedSeedFramework struct {
	*GardenerFramework
	TestDescription
	Config *ManagedSeedConfig

	ManagedSeed *seedmanagementv1alpha1.ManagedSeed
}

// ManagedSeedConfig is a managed seed framework configuration.
type ManagedSeedConfig struct {
	GardenerConfig  *GardenerConfig
	ManagedSeedName string
	ShootName       string
	DeployGardenlet bool
	BackupProvider  string
}

// NewManagedSeedFramework creates a new managed seed framework.
func NewManagedSeedFramework(cfg *ManagedSeedConfig) *ManagedSeedFramework {
	var gardenerConfig *GardenerConfig
	if cfg != nil {
		gardenerConfig = cfg.GardenerConfig
	}

	f := &ManagedSeedFramework{
		GardenerFramework: NewGardenerFrameworkFromConfig(gardenerConfig),
		TestDescription:   NewTestDescription("MANAGEDSEED"),
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

// BeforeEach should be called in ginkgo's BeforeEach.
// It merges and validates the managed seed framework configuration.
func (f *ManagedSeedFramework) BeforeEach(ctx context.Context) {
	f.Config = mergeManagedSeedConfig(f.Config, managedSeedConfig)
	validateManagedSeedConfig(f.Config)
}

// AfterEach should be called in ginkgo's AfterEach.
// It dumps the managed seed framework state if the test failed.
func (f *ManagedSeedFramework) AfterEach(ctx context.Context) {
	if ginkgo.CurrentGinkgoTestDescription().Failed {
		f.DumpState(ctx)
	}
}

func validateManagedSeedConfig(cfg *ManagedSeedConfig) {
	if cfg == nil {
		ginkgo.Fail("no configuration provided")
		return // make linters happy
	}
	if !StringSet(cfg.ManagedSeedName) {
		ginkgo.Fail("You should specify a name for the managed seed")
	}
	if !StringSet(cfg.ShootName) {
		ginkgo.Fail("You should specify the name of the shoot to be registered as seed")
	}
}

func mergeManagedSeedConfig(base, overwrite *ManagedSeedConfig) *ManagedSeedConfig {
	if base == nil {
		return overwrite
	}
	if overwrite == nil {
		return base
	}
	if overwrite.GardenerConfig != nil {
		base.GardenerConfig = overwrite.GardenerConfig
	}
	if StringSet(overwrite.ManagedSeedName) {
		base.ManagedSeedName = overwrite.ManagedSeedName
	}
	if StringSet(overwrite.ShootName) {
		base.ShootName = overwrite.ShootName
	}
	if overwrite.DeployGardenlet {
		base.DeployGardenlet = overwrite.DeployGardenlet
	}
	if StringSet(overwrite.BackupProvider) {
		base.BackupProvider = overwrite.BackupProvider
	}
	return base
}

// RegisterManagedSeedFrameworkFlags adds all flags that are needed to configure a managed seed framework.
func RegisterManagedSeedFrameworkFlags() *ManagedSeedConfig {
	_ = RegisterGardenerFrameworkFlags()

	newCfg := &ManagedSeedConfig{}

	flag.StringVar(&newCfg.ManagedSeedName, "managed-seed-name", "", "name of the managed seed")
	flag.StringVar(&newCfg.ShootName, "shoot-name", "", "name of the shoot to be registered as seed")
	flag.BoolVar(&newCfg.DeployGardenlet, "deploy-gardenlet", true, "indicates if gardenlet should be deployed to the shoot, default is true")
	flag.StringVar(&newCfg.BackupProvider, "backup-provider", "", "seed backup provider")

	managedSeedConfig = newCfg

	return managedSeedConfig
}

// CreateManagedSeed creates a new managed seed according to the managed seed framework configuration.
func (f *ManagedSeedFramework) CreateManagedSeed(ctx context.Context) error {
	if f.GardenClient == nil {
		return errors.New("no gardener client is defined")
	}

	// Get shoot
	shoot := &gardencorev1beta1.Shoot{}
	if err := f.GardenClient.Client().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, f.Config.ShootName), shoot); err != nil {
		return err
	}

	// Ensure shoot VPA is enabled
	if err := f.GardenerFramework.UpdateShoot(ctx, shoot, func(shoot *gardencorev1beta1.Shoot) error {
		shoot.Spec.Kubernetes.VerticalPodAutoscaler = &gardencorev1beta1.VerticalPodAutoscaler{
			Enabled: true,
		}
		return nil
	}); err != nil {
		return err
	}

	// Build managed seed object
	var err error
	if f.ManagedSeed, err = f.buildManagedSeed(); err != nil {
		return err
	}

	// Create managed seed and wait until it's successfully reconciled
	f.Logger.Infof("Creating managed seed %s", f.ManagedSeed.Name)
	if err := PrettyPrintObject(f.ManagedSeed); err != nil {
		return err
	}
	if err := f.GardenerFramework.CreateManagedSeed(ctx, f.ManagedSeed); err != nil {
		return err
	}

	// Wait until the seed is also successfully reconciled
	f.Logger.Infof("Waiting until seed %s is successfully reconciled", f.ManagedSeed.Name)
	seed := gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			Name: f.ManagedSeed.Name,
		},
	}
	return f.GardenerFramework.WaitForSeedToBeCreated(ctx, &seed)
}

func (f *ManagedSeedFramework) buildManagedSeed() (*seedmanagementv1alpha1.ManagedSeed, error) {
	var (
		seedTemplate *gardencorev1beta1.SeedTemplate
		gardenlet    *seedmanagementv1alpha1.Gardenlet
	)

	// Build seed spec
	seedSpec := BuildSeedSpecForTestrun(gutil.ComputeGardenNamespace(f.Config.ManagedSeedName), &f.Config.BackupProvider)

	if !f.Config.DeployGardenlet {
		// Initialize seed template
		seedTemplate = &gardencorev1beta1.SeedTemplate{
			Spec: *seedSpec,
		}
	} else {
		// Initialize gardenlet config
		config := &configv1alpha1.GardenletConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: configv1alpha1.SchemeGroupVersion.String(),
				Kind:       "GardenletConfiguration",
			},
			SeedConfig: &configv1alpha1.SeedConfig{
				SeedTemplate: gardencorev1beta1.SeedTemplate{
					Spec: *seedSpec,
				},
			},
		}

		// Encode gardenlet config to raw extension
		re, err := encoding.EncodeGardenletConfiguration(config)
		if err != nil {
			return nil, err
		}

		// Initialize gardenlet configuraton and parameters
		gardenlet = &seedmanagementv1alpha1.Gardenlet{
			Config: *re,
		}
	}

	return &seedmanagementv1alpha1.ManagedSeed{
		ObjectMeta: metav1.ObjectMeta{
			Name:      f.Config.ManagedSeedName,
			Namespace: v1beta1constants.GardenNamespace,
		},
		Spec: seedmanagementv1alpha1.ManagedSeedSpec{
			Shoot: &seedmanagementv1alpha1.Shoot{
				Name: f.Config.ShootName,
			},
			SeedTemplate: seedTemplate,
			Gardenlet:    gardenlet,
		},
	}, nil
}
