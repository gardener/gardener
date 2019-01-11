// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controllerinstallation

import (
	corev1 "k8s.io/api/core/v1"
)

// HelmDeployment is a providerConfig specific type for ControllerInstallation.
type HelmDeployment struct {
	// Chart is a Helm chart tarball.
	Chart []byte `json:"chart,omitempty"`
	// Values is a map of values for the given chart.
	Values map[string]interface{} `json:"values,omitempty"`
}

// DeployedResources is a providerStatus specific type for ControllerInstallation.
type DeployedResources struct {
	// Resources is a list of objects that have been created.
	Resources []corev1.ObjectReference `json:"resources,omitempty"`
}
