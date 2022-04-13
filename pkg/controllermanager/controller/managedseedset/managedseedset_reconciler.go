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
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// reconciler implements the reconcile.Reconciler interface for ManagedSeedSet reconciliation.
type reconciler struct {
	gardenClient kubernetes.Interface
	actuator     Actuator
	cfg          *config.ManagedSeedSetControllerConfiguration
}

// NewReconciler creates and returns a new ManagedSeedSet reconciler with the given parameters.
func NewReconciler(gardenClient kubernetes.Interface, actuator Actuator, cfg *config.ManagedSeedSetControllerConfiguration) reconcile.Reconciler {
	return &reconciler{
		gardenClient: gardenClient,
		actuator:     actuator,
		cfg:          cfg,
	}
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	set := &seedmanagementv1alpha1.ManagedSeedSet{}
	if err := r.gardenClient.Client().Get(ctx, request.NamespacedName, set); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if set.DeletionTimestamp != nil {
		return r.delete(ctx, log, set)
	}
	return r.reconcile(ctx, log, set)
}

func (r *reconciler) reconcile(ctx context.Context, log logr.Logger, set *seedmanagementv1alpha1.ManagedSeedSet) (result reconcile.Result, err error) {
	// Ensure gardener finalizer
	if !controllerutil.ContainsFinalizer(set, gardencorev1beta1.GardenerName) {
		log.V(1).Info("Adding finalizer")
		if err := controllerutils.StrategicMergePatchAddFinalizers(ctx, r.gardenClient.Client(), set, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not add finalizer to ManagedSeedSet: %w", err)
		}
		return reconcile.Result{}, nil
	}

	var status *seedmanagementv1alpha1.ManagedSeedSetStatus
	defer func() {
		// Update status, on failure return the update error unless there is another error
		if updateErr := r.updateStatus(ctx, set, status); updateErr != nil && err == nil {
			err = fmt.Errorf("could not update status: %w", updateErr)
		}
	}()

	// Reconcile creation or update
	log.V(1).Info("Reconciling creation or update")
	if status, _, err = r.actuator.Reconcile(ctx, log, set); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not reconcile ManagedSeedSet %s creation or update: %w", kutil.ObjectName(set), err)
	}
	log.V(1).Info("Creation or update reconciled")

	// Return success result
	return reconcile.Result{RequeueAfter: r.cfg.SyncPeriod.Duration}, nil
}

func (r *reconciler) delete(ctx context.Context, log logr.Logger, set *seedmanagementv1alpha1.ManagedSeedSet) (result reconcile.Result, err error) {
	// Check gardener finalizer
	if !controllerutil.ContainsFinalizer(set, gardencorev1beta1.GardenerName) {
		log.V(1).Info("Skipping as it does not have a finalizer")
		return reconcile.Result{}, nil
	}

	var status *seedmanagementv1alpha1.ManagedSeedSetStatus
	var removeFinalizer bool
	defer func() {
		// Only update status if the finalizer is not removed to prevent errors if the object is already gone
		if !removeFinalizer {
			// Update status, on failure return the update error unless there is another error
			if updateErr := r.updateStatus(ctx, set, status); updateErr != nil && err == nil {
				err = fmt.Errorf("could not update status: %w", updateErr)
			}
		}
	}()

	// Reconcile deletion
	log.V(1).Info("Reconciling deletion")
	if status, removeFinalizer, err = r.actuator.Reconcile(ctx, log, set); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not reconcile ManagedSeedSet %s deletion: %w", kutil.ObjectName(set), err)
	}
	log.V(1).Info("Deletion reconciled")

	// Remove gardener finalizer if requested by the actuator
	if removeFinalizer {
		log.V(1).Info("Removing finalizer")
		if err := controllerutils.PatchRemoveFinalizers(ctx, r.gardenClient.Client(), set, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not remove gardener finalizer: %w", err)
		}
		return reconcile.Result{}, nil
	}

	// Return success result
	return reconcile.Result{RequeueAfter: r.cfg.SyncPeriod.Duration}, nil
}

func (r *reconciler) updateStatus(ctx context.Context, set *seedmanagementv1alpha1.ManagedSeedSet, status *seedmanagementv1alpha1.ManagedSeedSetStatus) error {
	if status == nil {
		return nil
	}
	patch := client.StrategicMergeFrom(set.DeepCopy())
	set.Status = *status
	return r.gardenClient.Client().Status().Patch(ctx, set, patch)
}
