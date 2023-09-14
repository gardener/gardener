// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/gardener/gardener/pkg/api/extensions"
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
		CreateFunc: func(event event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(event event.UpdateEvent) bool {
			return statusHasChanged(event.ObjectOld, event.ObjectNew)
		},
		GenericFunc: func(event event.GenericEvent) bool {
			return false
		},
		DeleteFunc: func(event event.DeleteEvent) bool {
			return true
		},
	}
}

// WorkerSkipStateUpdateAnnotation is a Worker annotation that instructs the worker-state controller to do not reconcile the corresponding Worker.
const WorkerSkipStateUpdateAnnotation = "worker.gardener.cloud/skip-state-update"

// WorkerStateUpdateIsNotSkipped is a predicate deciding whether the Worker is not annotated to skip state updates.
func WorkerStateUpdateIsNotSkipped() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		acc, err := extensions.Accessor(obj)
		if err != nil {
			// We assume by default that the Worker is not annotated to skip state updates.
			return true
		}

		_, found := acc.GetAnnotations()[WorkerSkipStateUpdateAnnotation]
		return !found
	})
}
