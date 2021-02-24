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

package app

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	gardenlet "github.com/gardener/gardener/landscaper/gardenlet/pkg"
	"github.com/gardener/gardener/landscaper/gardenlet/pkg/apis/imports"
	importsv1alpha1 "github.com/gardener/gardener/landscaper/gardenlet/pkg/apis/imports/v1alpha1"
	importvalidation "github.com/gardener/gardener/landscaper/gardenlet/pkg/apis/imports/validation"

	gardenletconfig "github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/version/verflag"
	landscaperconstants "github.com/gardener/landscaper/apis/deployer/container"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)


// in integration tests, we do not assume that the Gardenlet can be rolled out successfully,
// nor that the Seed can be registered.
// this is to provide an easy means of testing the landscaper component without requiring
// a fully functional Gardener control plane.
var isIntegrationTest bool

// NewCommandStartLandscaperGardenelet creates a *cobra.Command object with default parameters
func NewCommandStartLandscaperGardenelet(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "landscaper-gardenlet",
		Short: "Launch the landscaper component for the Gardenlet.",
		Long:  "Gardener landscaper component for the Gardenlet. Sets up the Garden cluster and deploys the Gardenlet with TLS bootstrapping to automatically register the configured Seed cluster.",
		RunE: func(cmd *cobra.Command, args []string) error {
			verflag.PrintAndExitIfRequested()

			if len(args) != 0 && !isIntegrationTest {
				return errors.New("arguments are not supported. Please only set the path to the configuration file via environment variable \"IMPORTS_PATH\"")
			}

			return run(ctx)
		},
	}

	cmd.Flags().BoolVar(
		&isIntegrationTest,
		"integration-test",
		false,
		"component executed during integration test. This disables certain checks that require Gardener API groups in the Garden cluster.")

	// add version flag
	flags := cmd.Flags()
	verflag.AddFlags(flags)
	return cmd
}

func run(ctx context.Context) error {
	landscaperOperation, importPath, componentDescriptorPath, err := getLandscaperEnvironmentVariables()
	if err != nil {
		return err
	}

	imports, err := loadImportsFromFile(importPath)
	if err != nil {
		return fmt.Errorf("unable to load landscaper imports: %v", err)
	}

	if errs := importvalidation.ValidateLandscaperImports(imports); len(errs) > 0 {
		return fmt.Errorf("errors validating the landscaper imports: %+v", errs)
	}

	landscaper, err := gardenlet.NewGardenletLandscaper(imports, landscaperOperation, componentDescriptorPath, isIntegrationTest)
	if err != nil {
		return err
	}

	return landscaper.Run(ctx)
}

func getLandscaperEnvironmentVariables() (string, string, string, error) {
	var operation string
	if operation = os.Getenv(landscaperconstants.OperationName); operation != string(landscaperconstants.OperationReconcile) && operation != string(landscaperconstants.OperationDelete) {
		return "", "", "", fmt.Errorf("environment variable \"%s\" has to be set and must either be \"%s\" or \"%s\"", landscaperconstants.OperationName, landscaperconstants.OperationReconcile, landscaperconstants.OperationDelete)
	}

	var importPath, componentDescriptorPath string

	if importPath = os.Getenv(landscaperconstants.ImportsPathName); importPath == "" {
		return "", "", "", fmt.Errorf("environment variable \"%s\" has to be set and point to the file containing the configuration for the Gardenlet landscaper", landscaperconstants.ImportsPathName)
	}

	if componentDescriptorPath = os.Getenv(landscaperconstants.ComponentDescriptorPathName); componentDescriptorPath == "" {
		return "", "", "", fmt.Errorf("environment variable \"%s\" has to be set and point to the file containing the component descriptor for the Gardenlet landscaper", landscaperconstants.ComponentDescriptorPathName)
	}

	return operation, importPath, componentDescriptorPath, nil
}

// loadImportsFromFile loads the content of file and decodes it as a
// Imports object.
func loadImportsFromFile(file string) (*imports.Imports, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	landscaperImport := &imports.Imports{}

	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)

	if err := imports.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := importsv1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	// Adding internal and v1alpha1 Gardenlet types
	// Required to parse the Gardenlet component config
	if err := gardenletconfigv1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := gardenletconfig.AddToScheme(scheme); err != nil {
		return nil, err
	}

	if _, _, err := codecs.UniversalDecoder().Decode(data, nil, landscaperImport); err != nil {
		return nil, err
	}

	return landscaperImport, nil
}
