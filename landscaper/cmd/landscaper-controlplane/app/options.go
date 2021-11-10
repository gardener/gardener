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
	"fmt"
	"os"
	"path/filepath"

	landscaperutils "github.com/gardener/gardener/landscaper/common/utils"
	landscaperconstants "github.com/gardener/landscaper/apis/deployer/container"
	"github.com/spf13/pflag"
)

// Options has all the context and parameters needed to run the virtual garden deployer.
type Options struct {
	// OperationType is the operation to be executed.
	OperationType landscaperconstants.OperationType
	// ImportsPath is the path to the imports file.
	ImportsPath string
	// ExportsPath is the path to the exports file. The parent directory exists; the export file itself must be created.
	// The format of the exports file must be json or yaml.
	ExportsPath string
	// ComponentDescriptorPath is the path to the component descriptor file.
	ComponentDescriptorPath string
}

// NewOptions returns a new options structure.
func NewOptions() *Options {
	return &Options{OperationType: landscaperconstants.OperationReconcile}
}

// AddFlags adds flags for a specific Scheduler to the specified FlagSet.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
}

// InitializeFromEnvironment initializes the options from the found environment variables.
func (o *Options) InitializeFromEnvironment() error {
	var err error
	o.ExportsPath = os.Getenv("EXPORTS_PATH")
	o.OperationType, o.ImportsPath, o.ComponentDescriptorPath, err = landscaperutils.GetLandscaperEnvironmentVariables()
	if err != nil {
		return err
	}
	return nil
}

// validate validates control plane specific options.
func (o *Options) validate() error {
	if len(o.ExportsPath) == 0 {
		return fmt.Errorf("missing path for exports file")
	}

	folderPath := filepath.Dir(o.ExportsPath)
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		if err := os.MkdirAll(folderPath, 0700); err != nil {
			return fmt.Errorf("the export path is invalid: %w", err)
		}
	}

	return nil
}
