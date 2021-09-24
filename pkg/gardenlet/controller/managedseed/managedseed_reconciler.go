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

package managedseed

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/logger"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// reconciler implements the reconcile.Reconciler interface for ManagedSeed reconciliation.
type reconciler struct {
	gardenClient kubernetes.Interface
	actuator     Actuator
	cfg          *config.ManagedSeedControllerConfiguration
	logger       *logrus.Logger
}

// newReconciler creates a new ManagedSeed reconciler with the given parameters.
func newReconciler(gardenClient kubernetes.Interface, actuator Actuator, cfg *config.ManagedSeedControllerConfiguration, logger *logrus.Logger) reconcile.Reconciler {
	return &reconciler{
		gardenClient: gardenClient,
		actuator:     actuator,
		cfg:          cfg,
		logger:       logger,
	}
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	ms := &seedmanagementv1alpha1.ManagedSeed{}
	if err := r.gardenClient.Client().Get(ctx, request.NamespacedName, ms); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Debugf("Skipping ManagedSeed %s because it has been deleted", request.NamespacedName)
			return reconcile.Result{}, nil
		}
		r.logger.Errorf("Could not get ManagedSeed %s from store: %+v", request.NamespacedName, err)
		return reconcile.Result{}, err
	}

	if ms.DeletionTimestamp != nil {
		return r.delete(ctx, ms)
	}
	return r.reconcile(ctx, ms)
}

func (r *reconciler) reconcile(ctx context.Context, ms *seedmanagementv1alpha1.ManagedSeed) (result reconcile.Result, err error) {
	// Ensure gardener finalizer
	if !controllerutil.ContainsFinalizer(ms, gardencorev1beta1.GardenerName) {
		r.getLogger(ms).Debug("Adding finalizer")
		if err := controllerutils.StrategicMergePatchAddFinalizers(ctx, r.gardenClient.Client(), ms, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not ensure gardener finalizer: %w", err)
		}
	}

	var status *seedmanagementv1alpha1.ManagedSeedStatus
	defer func() {
		// Update status, on failure return the update error unless there is another error
		if updateErr := r.updateStatus(ctx, ms, status); updateErr != nil && err == nil {
			err = fmt.Errorf("could not update status: %w", updateErr)
		}
	}()

	// Reconcile creation or update
	r.getLogger(ms).Debug("Reconciling creation or update")
	var wait bool
	if status, wait, err = r.actuator.Reconcile(ctx, ms); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not reconcile ManagedSeed %s creation or update: %w", kutil.ObjectName(ms), err)
	}
	r.getLogger(ms).Debug("Creation or update reconciled")

	// If waiting, requeue after WaitSyncPeriod
	if wait {
		return reconcile.Result{RequeueAfter: r.cfg.WaitSyncPeriod.Duration}, nil
	}

	// Return success result
	return reconcile.Result{RequeueAfter: r.cfg.SyncPeriod.Duration}, nil
}

func (r *reconciler) delete(ctx context.Context, ms *seedmanagementv1alpha1.ManagedSeed) (result reconcile.Result, err error) {
	// Check gardener finalizer
	if !controllerutil.ContainsFinalizer(ms, gardencorev1beta1.GardenerName) {
		r.getLogger(ms).Debug("Skipping as it does not have a finalizer")
		return reconcile.Result{}, nil
	}

	var status *seedmanagementv1alpha1.ManagedSeedStatus
	var wait, removeFinalizer bool
	defer func() {
		// Only update status if the finalizer is not removed to prevent errors if the object is already gone
		if !removeFinalizer {
			// Update status, on failure return the update error unless there is another error
			if updateErr := r.updateStatus(ctx, ms, status); updateErr != nil && err == nil {
				err = fmt.Errorf("could not update status: %w", updateErr)
			}
		}
	}()

	// Reconcile deletion
	r.getLogger(ms).Debug("Reconciling deletion")
	if status, wait, removeFinalizer, err = r.actuator.Delete(ctx, ms); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not reconcile ManagedSeed %s deletion: %w", kutil.ObjectName(ms), err)
	}
	r.getLogger(ms).Debug("Deletion reconciled")

	// If waiting, requeue after WaitSyncPeriod
	if wait {
		return reconcile.Result{RequeueAfter: r.cfg.WaitSyncPeriod.Duration}, nil
	}

	// Remove gardener finalizer if requested by the actuator
	if removeFinalizer {
		r.getLogger(ms).Debug("Removing finalizer")
		if err := controllerutils.PatchRemoveFinalizers(ctx, r.gardenClient.Client(), ms, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not remove gardener finalizer: %w", err)
		}
		return reconcile.Result{}, nil
	}

	// Return success result
	return reconcile.Result{RequeueAfter: r.cfg.SyncPeriod.Duration}, nil
}

func (r *reconciler) getLogger(ms *seedmanagementv1alpha1.ManagedSeed) *logrus.Entry {
	return logger.NewFieldLogger(r.logger, "managedSeed", kutil.ObjectName(ms))
}

func (r *reconciler) updateStatus(ctx context.Context, ms *seedmanagementv1alpha1.ManagedSeed, status *seedmanagementv1alpha1.ManagedSeedStatus) error {
	if status == nil {
		return nil
	}
	patch := client.StrategicMergeFrom(ms.DeepCopy())
	ms.Status = *status
	return r.gardenClient.Client().Status().Patch(ctx, ms, patch)
}
