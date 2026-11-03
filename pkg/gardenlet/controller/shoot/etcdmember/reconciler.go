// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package etcdmember

import (
	"context"
	"fmt"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// Reconciler ensures the control plane machine's address is added to Etcd's `.spec.externallyManagedMemberAddresses[]`
// list and a finalizer is placed on the machine. When instance deletion is suspended (machine is being deleted and
// MCM is waiting for our signal), it removes the address from ETCD, then removes the finalizer and the suspend
// annotation so MCM can proceed with deleting the instance.
type Reconciler struct {
	Client   client.Client
	Clock    clock.Clock
	Recorder events.EventRecorder
}

// Reconcile ensures the control plane machine's address is added to Etcd's `.spec.externallyManagedMemberAddresses[]`
// list and manages the machine finalizer for orderly ETCD membership cleanup on deletion.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	machine := &machinev1alpha1.Machine{}
	if err := r.Client.Get(ctx, request.NamespacedName, machine); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	address, found := gardenerutils.PreferredNodeAddress(machine.Status.Addresses)
	if !found {
		log.Info("No address found in machine's .status.addresses[] list, exiting early")
		return reconcile.Result{}, nil
	}
	log = log.WithValues("address", address)

	if machine.DeletionTimestamp == nil {
		if err := controllerutils.AddFinalizers(ctx, r.Client, machine, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed adding finalizer to machine: %w", err)
		}

		log.Info("Ensure machine address is added to ETCD objects")
		if err := botanist.EnsureMachineIPInBothETCDs(log, r.Client, r.Clock, machine.Namespace, address, botanist.AddAddressToETCD)(ctx); err != nil {
			return reconcile.Result{}, err
		}
		r.Recorder.Eventf(machine, nil, corev1.EventTypeNormal, "AddedToETCD", "Reconcile", "Added machine address %s to ETCD objects", address)
		return reconcile.Result{}, nil
	}

	if instanceDeletionSuspended(machine) {
		log.Info("Ensure machine address is removed from ETCD objects")
		if err := botanist.EnsureMachineIPInBothETCDs(log, r.Client, r.Clock, machine.Namespace, address, botanist.RemoveAddressFromETCD)(ctx); err != nil {
			return reconcile.Result{}, err
		}
		r.Recorder.Eventf(machine, nil, corev1.EventTypeNormal, "RemovedFromETCD", "Reconcile", "Removed machine address %s from ETCD objects", address)

		log.Info("Machine IP removed from ETCD objects, removing suspend annotation and finalizer", "annotationKey", gardenerutils.AnnotationKeySuspendInstanceDeletion)
		patch := client.MergeFrom(machine.DeepCopy())
		delete(machine.Annotations, gardenerutils.AnnotationKeySuspendInstanceDeletion)
		controllerutil.RemoveFinalizer(machine, FinalizerName)
		if err := r.Client.Patch(ctx, machine, patch); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove suspend annotation from machine: %w", err)
		}
		r.Recorder.Eventf(machine, nil, corev1.EventTypeNormal, "SuspendAnnotationRemoved", "Reconcile", "Removed %s annotation from machine", gardenerutils.AnnotationKeySuspendInstanceDeletion)
	}

	return reconcile.Result{}, nil
}
