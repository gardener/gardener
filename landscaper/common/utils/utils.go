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

package utils

import (
	"fmt"
	"os"

	landscaperconstants "github.com/gardener/landscaper/apis/deployer/container"
)

// GetLandscaperEnvironmentVariables reads landscaper-specific environment variables and returns their values
//  returns in order: landscaper operation, import configuration path, component descriptor path
func GetLandscaperEnvironmentVariables() (landscaperconstants.OperationType, string, string, error) {
	var operation string
	if operation = os.Getenv(landscaperconstants.OperationName); operation != string(landscaperconstants.OperationReconcile) && operation != string(landscaperconstants.OperationDelete) {
		return "", "", "", fmt.Errorf("environment variable %q has to be set and must either be %q or %q", landscaperconstants.OperationName, landscaperconstants.OperationReconcile, landscaperconstants.OperationDelete)
	}

	var importPath, componentDescriptorPath string

	if importPath = os.Getenv(landscaperconstants.ImportsPathName); importPath == "" {
		return "", "", "", fmt.Errorf("environment variable %q has to be set and point to the file containing the configuration for the controlplane landscaper", landscaperconstants.ImportsPathName)
	}

	if componentDescriptorPath = os.Getenv(landscaperconstants.ComponentDescriptorPathName); componentDescriptorPath == "" {
		return "", "", "", fmt.Errorf("environment variable %q has to be set and point to the file containing the component descriptor for the controlplane landscaper", landscaperconstants.ComponentDescriptorPathName)
	}

	return landscaperconstants.OperationType(operation), importPath, componentDescriptorPath, nil
}
