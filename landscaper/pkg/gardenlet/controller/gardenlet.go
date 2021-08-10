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

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	v2 "github.com/gardener/component-spec/bindings-go/apis/v2"
	"github.com/gardener/component-spec/bindings-go/apis/v2/cdutils"
	"github.com/gardener/component-spec/bindings-go/codec"
	landscaperv1alpha1 "github.com/gardener/landscaper/apis/core/v1alpha1"
	landscaperconstants "github.com/gardener/landscaper/apis/deployer/container"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/charts"
	"github.com/gardener/gardener/landscaper/pkg/gardenlet/apis/imports"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
)

const (
	gardenerComponentName = "github.com/gardener/gardener"
	gardenletImageName    = "gardenlet"
)

// Landscaper has all the context and parameters needed to run a Gardenlet landscaper.
type Landscaper struct {
	log          *logrus.Entry
	gardenClient kubernetes.Interface
	seedClient   kubernetes.Interface
	// using internal version of the import configuration
	imports *imports.Imports
	// working on the external version of the GardenletConfiguration file
	gardenletConfiguration *configv1alpha1.GardenletConfiguration
	landscaperOperation    string

	chartPath string
	// disables certain checks that require Gardener API groups in the Garden cluster
	isIntegrationTest bool
	// the time the process should sleep to give the gardenlet process
	// time to startup and either fail or proceed
	rolloutSleepDuration time.Duration
}

// NewGardenletLandscaper creates a new Gardenlet landscaper.
func NewGardenletLandscaper(imports *imports.Imports, landscaperOperation, componentDescriptorPath string, isIntegrationTest bool) (*Landscaper, error) {
	// Get external gardenlet config from import configuration
	gardenletConfig, err := confighelper.ConvertGardenletConfigurationExternal(imports.ComponentConfiguration)
	if err != nil {
		return nil, err
	}

	gardenTargetConfig := &landscaperv1alpha1.KubernetesClusterTargetConfig{}
	if err := json.Unmarshal(imports.GardenCluster.Spec.Configuration.RawMessage, gardenTargetConfig); err != nil {
		return nil, fmt.Errorf("failed to parse the Garden cluster kubeconfig : %w", err)
	}

	// Create Garden client
	gardenClient, err := kubernetes.NewClientFromBytes([]byte(gardenTargetConfig.Kubeconfig), kubernetes.WithClientOptions(
		client.Options{
			Scheme: kubernetes.GardenScheme,
		}))
	if err != nil {
		return nil, fmt.Errorf("failed to create the Garden cluster client: %w", err)
	}

	seedClusterTargetConfig := &landscaperv1alpha1.KubernetesClusterTargetConfig{}
	if err := json.Unmarshal(imports.SeedCluster.Spec.Configuration.RawMessage, seedClusterTargetConfig); err != nil {
		return nil, fmt.Errorf("failed to parse the Runtime cluster kubeconfig: %w", err)
	}

	// Create Seed client
	seedClient, err := kubernetes.NewClientFromBytes([]byte(seedClusterTargetConfig.Kubeconfig))
	if err != nil {
		return nil, fmt.Errorf("failed to create the seed cluster client: %w", err)
	}

	landscaper := Landscaper{
		log:                    logger.NewFieldLogger(logger.NewLogger("info", ""), "landscaper-gardenlet operation", landscaperOperation),
		imports:                imports,
		gardenletConfiguration: gardenletConfig,
		landscaperOperation:    landscaperOperation,
		seedClient:             seedClient,
		gardenClient:           gardenClient,
		isIntegrationTest:      isIntegrationTest,
		chartPath:              charts.Path,
		rolloutSleepDuration:   10 * time.Second,
	}

	componentDescriptorData, err := os.ReadFile(componentDescriptorPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the Gardenlet component descriptor: %w", err)
	}

	componentDescriptorList := &v2.ComponentDescriptorList{}
	err = codec.Decode(componentDescriptorData, componentDescriptorList)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the Gardenlet component descriptor: %w", err)
	}

	imageReference, err := cdutils.GetImageReferenceFromList(componentDescriptorList, gardenerComponentName, gardenletImageName)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the component descriptor: %w", err)
	}

	repo, tag, _, err := cdutils.ParseImageReference(imageReference)
	if err != nil {
		return nil, err
	}

	// can safely dereference DeploymentConfiguration as it is defaulted
	landscaper.imports.DeploymentConfiguration.Image = &seedmanagement.Image{
		Repository: &repo,
		Tag:        &tag,
	}

	return &landscaper, nil
}

func (g Landscaper) Run(ctx context.Context) error {
	switch g.landscaperOperation {
	case string(landscaperconstants.OperationReconcile):
		return g.Reconcile(ctx)
	case string(landscaperconstants.OperationDelete):
		return g.Delete(ctx)
	default:
		return fmt.Errorf("environment variable \"OPERATION\" must either be set to %q or %q", landscaperconstants.OperationReconcile, landscaperconstants.OperationDelete)
	}
}
