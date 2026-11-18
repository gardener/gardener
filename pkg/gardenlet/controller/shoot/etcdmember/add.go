// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package etcdmember

import (
	"slices"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

const (
	// ControllerName is the name of this controller.
	ControllerName = "etcd-member"
	// FinalizerName is the finalizer placed on control plane machines to ensure ETCD membership is cleaned up before the
	// machine instance is deleted.
	FinalizerName = "gardenlet.gardener.cloud/etcd-member"
)

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorder(ControllerName + "-controller")
	}

	return builder.
		ControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}). // only one machine is updated a time for control plane pools
		Named(ControllerName).
		Watches(&machinev1alpha1.Machine{}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(
			r.ControlPlaneMachinePredicate(),
			predicate.Or(
				r.MachineAddressesPredicate(),
				r.InstanceDeletionSuspendedPredicate(),
			),
		)).
		Complete(r)
}

// ControlPlaneMachinePredicate returns true when the Machine object is annotated with the expected
// suspend-instance-deletion annotation.
func (r *Reconciler) ControlPlaneMachinePredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetAnnotations()[gardenerutils.AnnotationKeySuspendInstanceDeletion] == gardenerutils.AnnotationValueSuspendInstanceDeletion
	})
}

// MachineAddressesPredicate returns true when the `.status.addresses[]` in the old Machine object were empty and now
// got filled (at least one entry) in the new Machine object.
func (r *Reconciler) MachineAddressesPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(_ event.CreateEvent) bool { return true },
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldMachine, ok := e.ObjectOld.(*machinev1alpha1.Machine)
			if !ok {
				return false
			}
			newMachine, ok := e.ObjectNew.(*machinev1alpha1.Machine)
			if !ok {
				return false
			}

			return len(oldMachine.Status.Addresses) == 0 && len(newMachine.Status.Addresses) > 0
		},
	}
}

// InstanceDeletionSuspendedPredicate returns true when the InstanceDeletionSuspended condition was unset or False in
// the old Machine object and now got set to True in the new Machine object.
func (r *Reconciler) InstanceDeletionSuspendedPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(_ event.CreateEvent) bool { return true },
		DeleteFunc:  func(_ event.DeleteEvent) bool { return false },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldMachine, ok := e.ObjectOld.(*machinev1alpha1.Machine)
			if !ok {
				return false
			}
			newMachine, ok := e.ObjectNew.(*machinev1alpha1.Machine)
			if !ok {
				return false
			}

			return !instanceDeletionSuspended(oldMachine) && instanceDeletionSuspended(newMachine)
		},
	}
}

func instanceDeletionSuspended(machine *machinev1alpha1.Machine) bool {
	index := slices.IndexFunc(machine.Status.Conditions, func(condition corev1.NodeCondition) bool {
		// TODO(rfranzke): Use machinev1alpha1.ConditionInstanceDeletionSuspended once MCM is revendored.
		return condition.Type == "InstanceDeletionSuspended"
	})
	return index >= 0 && machine.Status.Conditions[index].Status == corev1.ConditionTrue
}
