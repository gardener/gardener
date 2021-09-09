// Copyright 2021 Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
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

package landscaper

import (
	"context"
	"flag"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/test/framework"
	landscaperv1alpha1 "github.com/gardener/landscaper/apis/core/v1alpha1"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var gardenletConfigFromFlags = &GardenletConfig{}

const (
	nameGardenletInstallation = "gardenlet"

	// resource names referenced from the Installation CRD
	cmNameGardenletLandscaperConfig      = "gardenlet-config"
	cmNameImageVectorOverwrite           = "gardenlet-image-vector-overwrite"
	cmNameComponentImageVectorOverwrites = "gardenlet-component-image-vector-overwrite"
	targetNameSeedCluster                = "seed-cluster"
	targetNameGardenCluster              = "garden-cluster"

	// data import keys in the Installation CRD
	// there are corresponding keys in the Blueprint
	dataImportComponentConfiguration         = "componentConfiguration"
	dataImportImageVectorOverwrite           = "imageVectorOverwrite"
	dataImportComponentImageVectorOverwrites = "componentImageVectorOverwrites"
	dataImportTargetSeed                     = "seedCluster"
	dataImportTargetGarden                   = "gardenCluster"
)

// GardenletFramework is a framework for the testing
// the gardenlet landscaper component.
type GardenletFramework struct {
	TestDescription
	*CommonFrameworkLandscaper
	*GardenerFramework

	Config     *GardenletConfig
	SeedClient client.Client

	// GardenletConfiguration contains the Gardenlet's configuration
	// that is contained in a config map and imported in the Installation CRD
	// to be read by the Landscaper gardenlet as part of its imports configuration
	// (landscaper/gardenlet/pkg/apis/imports/types.go)
	// exposed to create test cases based on the configuration
	ComponentConfiguration *configv1alpha1.GardenletConfiguration
}

// GardenletConfig is the configuration of the gardenlet landscaper component.
type GardenletConfig struct {
	SeedKubeconfigPath             string
	ImageVectorOverwrite           *string
	ComponentImageVectorOverwrites *string
	DeploymentConfiguration        *seedmanagementv1alpha1.GardenletDeployment
	GardenerConfig                 *GardenerConfig
	LandscaperCommonConfig         *LandscaperCommonConfig
}

// NewGardenletFramework creates a new GardenletFramework.
func NewGardenletFramework(cfg *GardenletConfig) *GardenletFramework {
	var gardenerConfig *GardenerConfig
	if cfg != nil {
		gardenerConfig = cfg.GardenerConfig
	}

	var landscaperCommonConfig *LandscaperCommonConfig
	if cfg != nil {
		landscaperCommonConfig = cfg.LandscaperCommonConfig
	}

	f := &GardenletFramework{
		GardenerFramework:         NewGardenerFrameworkFromConfig(gardenerConfig),
		CommonFrameworkLandscaper: NewLandscaperCommonFramework(landscaperCommonConfig),
		TestDescription:           NewTestDescription("LANDSCAPER_GARDENLET"),
		Config:                    cfg,
	}

	CBeforeEach(func(ctx context.Context) {
		f.GardenerFramework.BeforeEach()
		f.CommonFrameworkLandscaper.BeforeEach()
		f.BeforeEach()
	}, 8*time.Minute)

	return f
}

// RegisterGardenletFrameworkFlags adds all flags that are needed to
// configure a landscaper gardenlet framework to the provided flagset.
func RegisterGardenletFrameworkFlags() *GardenletConfig {
	_ = RegisterGardenerFrameworkFlags()
	_ = RegisterLandscaperCommonFrameworkFlags()

	newCfg := &GardenletConfig{}

	flag.StringVar(&newCfg.SeedKubeconfigPath, "seed-kubecfg-path", "", "the path to the kubeconfig  of the seed cluster that will be used for integration tests")

	gardenletConfigFromFlags = newCfg

	return gardenletConfigFromFlags
}

// BeforeEach should be called in ginkgo's BeforeEach.
// It merges the gardenet configuration with the configuration from flags
// and creates the seed client
func (f *GardenletFramework) BeforeEach() {
	f.Config = mergeGardenletConfig(f.Config, gardenletConfigFromFlags)

	validateGardenletConfig(f.Config)

	client, err := kubernetes.NewClientFromFile("", f.Config.SeedKubeconfigPath)

	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	f.SeedClient = client.Client()
}

func mergeGardenletConfig(base, overwrite *GardenletConfig) *GardenletConfig {
	if base == nil {
		return overwrite
	}

	if overwrite == nil {
		return base
	}

	if StringSet(overwrite.SeedKubeconfigPath) {
		base.SeedKubeconfigPath = overwrite.SeedKubeconfigPath
	}
	return base
}

func validateGardenletConfig(cfg *GardenletConfig) {
	if cfg == nil {
		ginkgo.Fail("no landscaper gardenlet configuration provided")
	}

	if !StringSet(cfg.SeedKubeconfigPath) {
		ginkgo.Fail("Need to set the kubeconfig path to the seed cluster")
	}
}

// CreateInstallationImports creates minimal import resources required or the Gardenlet landscaper
// - two targets
// - config map landscaper configuration
// - config map image vector override
// - config map components image vector override
func (f *GardenletFramework) CreateInstallationImports(ctx context.Context) error {
	err := f.CreateTargetWithKubeconfig(ctx, targetNameSeedCluster, f.Config.SeedKubeconfigPath)
	if err != nil {
		return err
	}

	err = f.CreateTargetWithKubeconfig(ctx, targetNameGardenCluster, f.GardenerFramework.GardenerFrameworkConfig.GardenerKubeconfig)
	if err != nil {
		return err
	}

	// config map containing the Gardenlet landscaper configuration
	err = f.createGardenletLandscaperConfig(ctx)
	if err != nil {
		return err
	}

	// image vectors are in dedicated config maps separate from the
	// Gardenlet landscaper configuration
	if f.Config.ImageVectorOverwrite != nil {
		data := map[string]string{
			dataImportImageVectorOverwrite: fmt.Sprintf(`|
%s`, *f.Config.ImageVectorOverwrite),
		}

		if err := f.CreateConfigMap(ctx, cmNameImageVectorOverwrite, data); err != nil {
			return err
		}
	}

	if f.Config.ComponentImageVectorOverwrites != nil {
		data := map[string]string{
			dataImportComponentImageVectorOverwrites: fmt.Sprintf(`|
%s`, *f.Config.ComponentImageVectorOverwrites),
		}

		if err := f.CreateConfigMap(ctx, cmNameComponentImageVectorOverwrites, data); err != nil {
			return err
		}
	}

	return nil
}

func (f *GardenletFramework) createGardenletLandscaperConfig(ctx context.Context) error {
	seedSpec := BuildSeedSpecForTestrun(gutil.ComputeGardenNamespace(f.ResourceSuffix), nil)

	// Initialize minimal Gardenlet config
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

	// remember the deployed configuration
	f.ComponentConfiguration = config

	// Encode gardenlet config to bytes
	re, err := encoding.EncodeGardenletConfigurationToBytes(config)
	if err != nil {
		return err
	}

	data := map[string]string{
		dataImportComponentConfiguration: string(re),
	}

	return f.CreateConfigMap(ctx, cmNameGardenletLandscaperConfig, data)
}

// CreateInstallation creates an Installation CRD in the landscaper cluster
func (f *GardenletFramework) CreateInstallation(ctx context.Context) (*landscaperv1alpha1.Installation, error) {
	installation := f.CommonFrameworkLandscaper.GetInstallation(nameGardenletInstallation)
	installation.Spec.Imports = landscaperv1alpha1.InstallationImports{
		Targets: []landscaperv1alpha1.TargetImportExport{
			{
				Name:   dataImportTargetGarden,
				Target: fmt.Sprintf("#%s", targetNameGardenCluster),
			},
			{
				Name:   dataImportTargetSeed,
				Target: fmt.Sprintf("#%s", targetNameSeedCluster),
			},
		},
		Data: []landscaperv1alpha1.DataImport{
			{
				Name: dataImportComponentConfiguration,
				ConfigMapRef: &landscaperv1alpha1.ConfigMapReference{
					ObjectReference: landscaperv1alpha1.ObjectReference{
						Name:      fmt.Sprintf("%s-%s", cmNameGardenletLandscaperConfig, f.ResourceSuffix),
						Namespace: f.CommonFrameworkLandscaper.Config.TargetNamespace,
					},
					Key: dataImportComponentConfiguration,
				},
			},
		},
	}

	if f.Config.ImageVectorOverwrite != nil {
		imageVectorOverwrite := landscaperv1alpha1.DataImport{
			Name: dataImportImageVectorOverwrite,
			ConfigMapRef: &landscaperv1alpha1.ConfigMapReference{
				ObjectReference: landscaperv1alpha1.ObjectReference{
					Name:      fmt.Sprintf("%s-%s", cmNameImageVectorOverwrite, f.ResourceSuffix),
					Namespace: f.CommonFrameworkLandscaper.Config.TargetNamespace,
				},
				Key: dataImportImageVectorOverwrite,
			},
		}
		installation.Spec.Imports.Data = append(installation.Spec.Imports.Data, imageVectorOverwrite)
	}

	if f.Config.ComponentImageVectorOverwrites != nil {
		componentsImageVectorOverwrite := landscaperv1alpha1.DataImport{
			Name: dataImportComponentImageVectorOverwrites,
			ConfigMapRef: &landscaperv1alpha1.ConfigMapReference{
				ObjectReference: landscaperv1alpha1.ObjectReference{
					Name:      fmt.Sprintf("%s-%s", cmNameComponentImageVectorOverwrites, f.ResourceSuffix),
					Namespace: f.CommonFrameworkLandscaper.Config.TargetNamespace,
				},
				Key: dataImportComponentImageVectorOverwrites,
			},
		}
		installation.Spec.Imports.Data = append(installation.Spec.Imports.Data, componentsImageVectorOverwrite)
	}

	if err := f.createInstallationAndWaitToBecomeHealthy(ctx, installation); err != nil {
		return nil, err
	}

	return installation, nil
}

// DeleteInstallationResources deletes the previously created landscaper resources
func (f *GardenletFramework) DeleteInstallationResources(ctx context.Context) error {
	if err := f.CommonFrameworkLandscaper.DeleteInstallation(ctx, nameGardenletInstallation); err != nil {
		return err
	}

	// delete targets
	if err := f.CommonFrameworkLandscaper.DeleteTarget(ctx, targetNameSeedCluster); err != nil {
		return err
	}

	if err := f.CommonFrameworkLandscaper.DeleteTarget(ctx, targetNameGardenCluster); err != nil {
		return err
	}

	// delete Landscaper Gardenlet config map
	if err := f.CommonFrameworkLandscaper.DeleteConfigMap(ctx, cmNameGardenletLandscaperConfig); err != nil {
		return err
	}

	// delete image vector config maps
	if f.Config.ImageVectorOverwrite != nil {
		if err := f.CommonFrameworkLandscaper.DeleteConfigMap(ctx, cmNameImageVectorOverwrite); err != nil {
			return err
		}
	}

	if f.Config.ComponentImageVectorOverwrites != nil {
		if err := f.CommonFrameworkLandscaper.DeleteConfigMap(ctx, cmNameComponentImageVectorOverwrites); err != nil {
			return err
		}
	}

	return nil
}
