// Copyright 2018 The Gardener Authors.
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

package azurebotanist

import (
	"github.com/gardener/gardener/pkg/operation"
)

// GetMachineClassInfo returns the name of the class kind, the plural of it and the name of the Helm chart which
// contains the machine class template.
// TODO@MachineController: Implement once the machine-controller-manager supports Azure.
func (b *AzureBotanist) GetMachineClassInfo() (classKind, classPlural, classChartName string) {
	classKind = ""
	classPlural = ""
	classChartName = ""
	return
}

// GenerateMachineConfig generates the configuration values for the cloud-specific machine class Helm chart. It
// also generates a list of corresponding MachineDeployments. It returns the name of the cloud-specific MachineClass, the cloud-specific Helm
// chart name, the corresponding values, and the list of MachineDeployments.
// TODO@MachineController: Implement once the machine-controller-manager supports Azure.
func (b *AzureBotanist) GenerateMachineConfig() ([]map[string]interface{}, []operation.MachineDeployment, error) {
	return nil, nil, nil
}
