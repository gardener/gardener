// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	utilerrors "github.com/gardener/gardener/pkg/utils/errors"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/hashicorp/go-multierror"
)

const (
	nameLabel = "name"
	// MachineSetKind is the kind of the owner reference of a machine set
	MachineSetKind = "MachineSet"
	// MachineDeploymentKind is the kind of the owner reference of a machine deployment
	MachineDeploymentKind = "MachineDeployment"
)

// BuildOwnerToMachinesMap builds a map from a slice of machinev1alpha1.Machine, that maps the owner reference
// to a slice of machines with the same owner reference
func BuildOwnerToMachinesMap(machines []machinev1alpha1.Machine) map[string][]machinev1alpha1.Machine {
	ownerToMachines := make(map[string][]machinev1alpha1.Machine)
	for index, machine := range machines {
		if len(machine.OwnerReferences) > 0 {
			for _, reference := range machine.OwnerReferences {
				if reference.Kind == MachineSetKind {
					ownerToMachines[reference.Name] = append(ownerToMachines[reference.Name], machines[index])
				}
			}
		} else if len(machine.Labels) > 0 {
			if machineDeploymentName, ok := machine.Labels[nameLabel]; ok {
				ownerToMachines[machineDeploymentName] = append(ownerToMachines[machineDeploymentName], machines[index])
			}
		}
	}
	return ownerToMachines
}

// BuildOwnerToMachineSetsMap builds a map from a slice of machinev1alpha1.MachineSet, that maps the owner reference
// to a slice of MachineSets with the same owner reference
func BuildOwnerToMachineSetsMap(machineSets []machinev1alpha1.MachineSet) map[string][]machinev1alpha1.MachineSet {
	ownerToMachineSets := make(map[string][]machinev1alpha1.MachineSet)
	for index, machineSet := range machineSets {
		if len(machineSet.OwnerReferences) > 0 {
			for _, reference := range machineSet.OwnerReferences {
				if reference.Kind == MachineDeploymentKind {
					ownerToMachineSets[reference.Name] = append(ownerToMachineSets[reference.Name], machineSets[index])
				}
			}
		} else if len(machineSet.Labels) > 0 {
			if machineDeploymentName, ok := machineSet.Labels[nameLabel]; ok {
				ownerToMachineSets[machineDeploymentName] = append(ownerToMachineSets[machineDeploymentName], machineSets[index])
			}
		}
	}
	return ownerToMachineSets
}

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
		ErrorFormat: utilerrors.NewErrorFormatFuncWithPrefix("machine(s) failed"),
	}
	for description, names := range descriptionPerFailedMachines {
		allErrs = multierror.Append(allErrs, fmt.Errorf("%s: %s", strings.Join(names, ", "), description))
	}

	return allErrs
}
