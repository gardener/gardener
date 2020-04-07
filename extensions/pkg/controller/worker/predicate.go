// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
