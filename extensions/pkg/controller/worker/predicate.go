// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// MachineStatusHasChanged is a predicate deciding wether the status of a MCM's Machine has been changed.
func MachineStatusHasChanged() predicate.Predicate {
	statusHasChanged := func(oldObj runtime.Object, newObj runtime.Object) bool {
		oldMachine, ok := oldObj.(*machinev1alpha1.Machine)
		if !ok {
			return false
		}
		newMachine, ok := newObj.(*machinev1alpha1.Machine)
		if !ok {
			return false
		}
		oldStatus := oldMachine.Status
		newStatus := newMachine.Status

		return oldStatus.Node != newStatus.Node
	}

	return predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(event event.UpdateEvent) bool {
			result := statusHasChanged(event.ObjectOld, event.ObjectNew)
			return result
		},
		GenericFunc: func(event event.GenericEvent) bool {
			return false
		},
		DeleteFunc: func(event event.DeleteEvent) bool {
			return true
		},
	}
}
