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

package backupbucket

import (
	"context"
	"fmt"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

type reconciler struct {
	actuator Actuator

	client        client.Client
	reader        client.Reader
	statusUpdater extensionscontroller.StatusUpdater
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// BackupBucket resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(actuator Actuator) reconcile.Reconciler {
	return reconcilerutils.OperationAnnotationWrapper(
		func() client.Object { return &extensionsv1alpha1.BackupBucket{} },
		&reconciler{
			actuator:      actuator,
			statusUpdater: extensionscontroller.NewStatusUpdater(),
		},
	)
}

func (r *reconciler) InjectFunc(f inject.Func) error {
	return f(r.actuator)
}

func (r *reconciler) InjectClient(client client.Client) error {
	r.client = client
	r.statusUpdater.InjectClient(client)
	return nil
}

func (r *reconciler) InjectAPIReader(reader client.Reader) error {
	r.reader = reader
	return nil
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	bb := &extensionsv1alpha1.BackupBucket{}
	if err := r.client.Get(ctx, request.NamespacedName, bb); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if bb.DeletionTimestamp != nil {
		return r.delete(ctx, log, bb)
	}

	return r.reconcile(ctx, log, bb)
}

func (r *reconciler) reconcile(ctx context.Context, log logr.Logger, bb *extensionsv1alpha1.BackupBucket) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(bb, FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.client, bb, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(bb.ObjectMeta, bb.Status.LastOperation)
	if err := r.statusUpdater.Processing(ctx, log, bb, operationType, "Reconciling the backupbucket"); err != nil {
		return reconcile.Result{}, err
	}

	secret, err := kutil.GetSecretByReference(ctx, r.client, &bb.Spec.SecretRef)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get backup bucket secret: %+v", err)
	}

	if !controllerutil.ContainsFinalizer(secret, FinalizerName) {
		log.Info("Adding finalizer to secret", "secret", client.ObjectKeyFromObject(secret))
		if err := controllerutils.AddFinalizers(ctx, r.client, secret, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer to secret: %w", err)
		}
	}

	log.Info("Starting the reconciliation of BackupBucket")
	if err := r.actuator.Reconcile(ctx, log, bb); err != nil {
		_ = r.statusUpdater.Error(ctx, log, bb, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Error reconciling backupbucket")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, bb, operationType, "Successfully reconciled backupbucket"); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, log logr.Logger, bb *extensionsv1alpha1.BackupBucket) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(bb, FinalizerName) {
		log.Info("Deleting BackupBucket causes a no-op as there is no finalizer")
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(bb.ObjectMeta, bb.Status.LastOperation)
	if err := r.statusUpdater.Processing(ctx, log, bb, operationType, "Deleting the BackupBucket"); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the deletion of BackupBucket")
	if err := r.actuator.Delete(ctx, log, bb); err != nil {
		_ = r.statusUpdater.Error(ctx, log, bb, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Error deleting BackupBucket")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, log, bb, operationType, "Successfully deleted BackupBucket"); err != nil {
		return reconcile.Result{}, err
	}

	secret, err := kutil.GetSecretByReference(ctx, r.client, &bb.Spec.SecretRef)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get backup bucket secret: %+v", err)
	}

	if controllerutil.ContainsFinalizer(secret, FinalizerName) {
		log.Info("Removing finalizer from secret", "secret", client.ObjectKeyFromObject(secret))
		if err := controllerutils.RemoveFinalizers(ctx, r.client, secret, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer from secret: %w", err)
		}
	}

	if controllerutil.ContainsFinalizer(bb, FinalizerName) {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.client, bb, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}
