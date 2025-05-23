// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// MachineNodeInfoHasChanged is a predicate deciding whether the information about the backing node of a Machine has
// been changed.
func MachineNodeInfoHasChanged() predicate.Predicate {
	statusHasChanged := func(oldObj client.Object, newObj client.Object) bool {
		oldMachine, ok := oldObj.(*machinev1alpha1.Machine)
		if !ok {
			return false
		}

		newMachine, ok := newObj.(*machinev1alpha1.Machine)
		if !ok {
			return false
		}

		return oldMachine.Spec.ProviderID != newMachine.Spec.ProviderID ||
			oldMachine.Labels[machinev1alpha1.NodeLabelKey] != newMachine.Labels[machinev1alpha1.NodeLabelKey]
	}

	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(event event.UpdateEvent) bool {
			return statusHasChanged(event.ObjectOld, event.ObjectNew)
		},
		GenericFunc: func(_ event.GenericEvent) bool {
			return false
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return true
		},
	}
}
