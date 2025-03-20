// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

// GetOldMachineSets returns all machine sets except the latest one.
func GetOldMachineSets(machineSets []machinev1alpha1.MachineSet, latestMachineSet machinev1alpha1.MachineSet) []machinev1alpha1.MachineSet {
	var oldMachineSets []machinev1alpha1.MachineSet
	for _, machineSet := range machineSets {
		if machineSet.Name != latestMachineSet.Name {
			oldMachineSets = append(oldMachineSets, machineSet)
		}
	}

	return oldMachineSets
}
