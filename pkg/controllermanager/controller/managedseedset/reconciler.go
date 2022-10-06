// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package managedseedset

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Reconciler reconciles the ManagedSeedSet.
type Reconciler struct {
	Client   client.Client
	Config   config.ManagedSeedSetControllerConfiguration
	Actuator Actuator
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	managedSeedSet := &seedmanagementv1alpha1.ManagedSeedSet{}
	if err := r.Client.Get(ctx, request.NamespacedName, managedSeedSet); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if managedSeedSet.DeletionTimestamp != nil {
		return r.delete(ctx, log, managedSeedSet)
	}
	return r.reconcile(ctx, log, managedSeedSet)
}

func (r *Reconciler) reconcile(ctx context.Context, log logr.Logger, managedSeedSet *seedmanagementv1alpha1.ManagedSeedSet) (result reconcile.Result, err error) {
	// Ensure gardener finalizer
	if !controllerutil.ContainsFinalizer(managedSeedSet, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.Client, managedSeedSet, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not add finalizer: %w", err)
		}
	}

	var status *seedmanagementv1alpha1.ManagedSeedSetStatus
	defer func() {
		// Update status, on failure return the update error unless there is another error
		if updateErr := r.updateStatus(ctx, managedSeedSet, status); updateErr != nil && err == nil {
			err = fmt.Errorf("could not update status: %w", updateErr)
		}
	}()

	// Reconcile creation or update
	log.V(1).Info("Reconciling creation or update")
	if status, _, err = r.Actuator.Reconcile(ctx, log, managedSeedSet); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not reconcile ManagedSeedSet %s creation or update: %w", kutil.ObjectName(managedSeedSet), err)
	}
	log.V(1).Info("Creation or update reconciled")

	// Return success result
	return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
}

func (r *Reconciler) delete(ctx context.Context, log logr.Logger, managedSeedSet *seedmanagementv1alpha1.ManagedSeedSet) (result reconcile.Result, err error) {
	// Check gardener finalizer
	if !controllerutil.ContainsFinalizer(managedSeedSet, gardencorev1beta1.GardenerName) {
		log.V(1).Info("Skipping as it does not have a finalizer")
		return reconcile.Result{}, nil
	}

	var status *seedmanagementv1alpha1.ManagedSeedSetStatus
	var removeFinalizer bool
	defer func() {
		// Only update status if the finalizer is not removed to prevent errors if the object is already gone
		if !removeFinalizer {
			// Update status, on failure return the update error unless there is another error
			if updateErr := r.updateStatus(ctx, managedSeedSet, status); updateErr != nil && err == nil {
				err = fmt.Errorf("could not update status: %w", updateErr)
			}
		}
	}()

	// Reconcile deletion
	log.V(1).Info("Reconciling deletion")
	if status, removeFinalizer, err = r.Actuator.Reconcile(ctx, log, managedSeedSet); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not reconcile ManagedSeedSet %s deletion: %w", kutil.ObjectName(managedSeedSet), err)
	}
	log.V(1).Info("Deletion reconciled")

	// Remove gardener finalizer if requested by the actuator
	if removeFinalizer {
		if controllerutil.ContainsFinalizer(managedSeedSet, gardencorev1beta1.GardenerName) {
			log.Info("Removing finalizer")
			if err := controllerutils.RemoveFinalizers(ctx, r.Client, managedSeedSet, gardencorev1beta1.GardenerName); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
			}
		}

		return reconcile.Result{}, nil
	}

	// Return success result
	return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
}

func (r *Reconciler) updateStatus(ctx context.Context, managedSeedSet *seedmanagementv1alpha1.ManagedSeedSet, status *seedmanagementv1alpha1.ManagedSeedSetStatus) error {
	if status == nil {
		return nil
	}
	patch := client.StrategicMergeFrom(managedSeedSet.DeepCopy())
	managedSeedSet.Status = *status
	return r.Client.Status().Patch(ctx, managedSeedSet, patch)
}
