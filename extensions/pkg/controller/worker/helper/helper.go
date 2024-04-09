// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package helper

import (
	"fmt"
	"strings"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/hashicorp/go-multierror"

	errorsutils "github.com/gardener/gardener/pkg/utils/errors"
)

// GetMachineSetWithMachineClass checks if for the given <machineDeploymentName>, there exists a machine set in the <ownerReferenceToMachineSet> with the machine class <machineClassName>
// returns the machine set or nil
func GetMachineSetWithMachineClass(machineDeploymentName, machineClassName string, ownerReferenceToMachineSet map[string][]machinev1alpha1.MachineSet) *machinev1alpha1.MachineSet {
	machineSets := ownerReferenceToMachineSet[machineDeploymentName]
	for _, machineSet := range machineSets {
		if machineSet.Spec.Template.Spec.Class.Name == machineClassName {
			return &machineSet
		}
	}
	return nil
}

// ReportFailedMachines reports the names of failed machines in the given `status` per error description.
func ReportFailedMachines(status machinev1alpha1.MachineDeploymentStatus) error {
	machines := status.FailedMachines
	if len(machines) == 0 {
		return nil
	}

	descriptionPerFailedMachines := make(map[string][]string)
	for _, machine := range machines {
		descriptionPerFailedMachines[machine.LastOperation.Description] = append(descriptionPerFailedMachines[machine.LastOperation.Description],
			fmt.Sprintf("%q", machine.Name))
	}

	allErrs := &multierror.Error{
		ErrorFormat: errorsutils.NewErrorFormatFuncWithPrefix("machine(s) failed"),
	}
	for description, names := range descriptionPerFailedMachines {
		allErrs = multierror.Append(allErrs, fmt.Errorf("%s: %s", strings.Join(names, ", "), description))
	}

	return allErrs
}
