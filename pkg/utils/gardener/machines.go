// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardener

import (
	"context"
	"fmt"
	"strings"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	retryutils "github.com/gardener/gardener/pkg/utils/retry"
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

// WaitUntilMachineResourcesDeleted waits for a maximum of 30 minutes until all machine resources have been properly
// deleted by the machine-controller-manager. It polls the status every 5 seconds.
func WaitUntilMachineResourcesDeleted(ctx context.Context, log logr.Logger, reader client.Reader, namespace string) error {
	var (
		countMachines            = -1
		countMachineSets         = -1
		countMachineDeployments  = -1
		countMachineClasses      = -1
		countMachineClassSecrets = -1
	)
	log.Info("Waiting until all machine resources have been deleted")

	return retryutils.UntilTimeout(ctx, 5*time.Second, 5*time.Minute, func(ctx context.Context) (bool, error) {
		var msg string

		// Check whether all machines have been deleted.
		if countMachines != 0 {
			machineList := &metav1.PartialObjectMetadataList{}
			machineList.SetGroupVersionKind(machinev1alpha1.SchemeGroupVersion.WithKind("MachineList"))
			if err := reader.List(ctx, machineList, client.InNamespace(namespace)); err != nil {
				return retryutils.SevereError(err)
			}
			countMachines = len(machineList.Items)
			msg += fmt.Sprintf("%d machines, ", countMachines)
		}

		// Check whether all machine sets have been deleted.
		if countMachineSets != 0 {
			machineSetList := &metav1.PartialObjectMetadataList{}
			machineSetList.SetGroupVersionKind(machinev1alpha1.SchemeGroupVersion.WithKind("MachineSetList"))
			if err := reader.List(ctx, machineSetList, client.InNamespace(namespace)); err != nil {
				return retryutils.SevereError(err)
			}
			countMachineSets = len(machineSetList.Items)
			msg += fmt.Sprintf("%d machine sets, ", countMachineSets)
		}

		// Check whether all machine deployments have been deleted.
		if countMachineDeployments != 0 {
			machineDeploymentList := &machinev1alpha1.MachineDeploymentList{}
			if err := reader.List(ctx, machineDeploymentList, client.InNamespace(namespace)); err != nil {
				return retryutils.SevereError(err)
			}
			countMachineDeployments = len(machineDeploymentList.Items)
			msg += fmt.Sprintf("%d machine deployments, ", countMachineDeployments)

			// Check whether an operation failed during the deletion process.
			for _, existingMachineDeployment := range machineDeploymentList.Items {
				for _, failedMachine := range existingMachineDeployment.Status.FailedMachines {
					return retryutils.SevereError(fmt.Errorf("machine %s failed: %s", failedMachine.Name, failedMachine.LastOperation.Description))
				}
			}
		}

		// Check whether all machine classes have been deleted.
		if countMachineClasses != 0 {
			machineClassList := &metav1.PartialObjectMetadataList{}
			machineClassList.SetGroupVersionKind(machinev1alpha1.SchemeGroupVersion.WithKind("MachineClassList"))
			if err := reader.List(ctx, machineClassList, client.InNamespace(namespace)); err != nil {
				return retryutils.SevereError(err)
			}
			countMachineClasses = len(machineClassList.Items)
			msg += fmt.Sprintf("%d machine classes, ", countMachineClasses)
		}

		// Check whether all machine class secrets have been deleted.
		if countMachineClassSecrets != 0 {
			count := 0
			machineClassSecretsList := &metav1.PartialObjectMetadataList{}
			machineClassSecretsList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("SecretList"))
			if err := reader.List(ctx, machineClassSecretsList, client.InNamespace(namespace), client.MatchingLabels(map[string]string{v1beta1constants.GardenerPurpose: v1beta1constants.GardenPurposeMachineClass})); err != nil {
				return retryutils.SevereError(err)
			}
			for _, machineClassSecret := range machineClassSecretsList.Items {
				if len(machineClassSecret.Finalizers) != 0 {
					count++
				}
			}
			countMachineClassSecrets = count
			msg += fmt.Sprintf("%d machine class secrets, ", countMachineClassSecrets)
		}

		if countMachines != 0 || countMachineSets != 0 || countMachineDeployments != 0 || countMachineClasses != 0 || countMachineClassSecrets != 0 {
			log.Info("Waiting until machine resources have been deleted",
				"machines", countMachines, "machineSets", countMachineSets, "machineDeployments", countMachineDeployments,
				"machineClasses", countMachineClasses, "machineClassSecrets", countMachineClassSecrets)
			return retryutils.MinorError(fmt.Errorf("waiting until the following machine resources have been deleted: %s", strings.TrimSuffix(msg, ", ")))
		}

		return retryutils.Ok()
	})
}
