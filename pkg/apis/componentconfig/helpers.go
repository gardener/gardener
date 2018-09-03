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

package componentconfig

import (
	"os"
)

// ApplyEnvironmentToConfig checks for several well-defined environment variables and if they are set,
// it sets the value of the respective keys of <config> to the values in the environment.
// Currently implemented environment variables:
// KUBECONFIG can override config.ClientConnection.KubeConfigFile
func ApplyEnvironmentToConfig(config *ControllerManagerConfiguration) {
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		config.ClientConnection.KubeConfigFile = kubeconfig
	}
	if kubeconfig := os.Getenv("GARDENER_KUBECONFIG"); kubeconfig != "" {
		config.GardenerClientConnection.KubeConfigFile = kubeconfig
	}
}
