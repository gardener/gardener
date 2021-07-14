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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

type reconciler struct {
	logger   logr.Logger
	actuator Actuator

	client        client.Client
	reader        client.Reader
	statusUpdater extensionscontroller.StatusUpdater
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// BackupBucket resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(actuator Actuator) reconcile.Reconciler {
	logger := log.Log.WithName(ControllerName)

	return extensionscontroller.OperationAnnotationWrapper(
		func() client.Object { return &extensionsv1alpha1.BackupBucket{} },
		&reconciler{
			logger:        logger,
			actuator:      actuator,
			statusUpdater: extensionscontroller.NewStatusUpdater(logger),
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
	bb := &extensionsv1alpha1.BackupBucket{}
	if err := r.client.Get(ctx, request.NamespacedName, bb); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if bb.DeletionTimestamp != nil {
		return r.delete(ctx, bb)
	}

	return r.reconcile(ctx, bb)
}

func (r *reconciler) reconcile(ctx context.Context, bb *extensionsv1alpha1.BackupBucket) (reconcile.Result, error) {
	if err := controllerutils.EnsureFinalizer(ctx, r.reader, r.client, bb, FinalizerName); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to ensure finalizer on backup bucket: %+v", err)
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(bb.ObjectMeta, bb.Status.LastOperation)
	if err := r.statusUpdater.Processing(ctx, bb, operationType, "Reconciling the backupbucket"); err != nil {
		return reconcile.Result{}, err
	}

	secret, err := kutil.GetSecretByReference(ctx, r.client, &bb.Spec.SecretRef)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get backup bucket secret: %+v", err)
	}
	if err := controllerutils.EnsureFinalizer(ctx, r.reader, r.client, secret, FinalizerName); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to ensure finalizer on bucket secret: %+v", err)
	}

	r.logger.Info("Starting the reconciliation of backupbucket", "backupbucket", kutil.ObjectName(bb))
	if err := r.actuator.Reconcile(ctx, bb); err != nil {
		_ = r.statusUpdater.Error(ctx, bb, extensionscontroller.ReconcileErrCauseOrErr(err), operationType, "Error reconciling backupbucket")
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, bb, operationType, "Successfully reconciled backupbucket"); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, bb *extensionsv1alpha1.BackupBucket) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(bb, FinalizerName) {
		r.logger.Info("Deleting backupbucket causes a no-op as there is no finalizer", "backupbucket", kutil.ObjectName(bb))
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(bb.ObjectMeta, bb.Status.LastOperation)
	if err := r.statusUpdater.Processing(ctx, bb, operationType, "Deleting the backupbucket"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the deletion of backupbucket", "backupbucket", kutil.ObjectName(bb))
	if err := r.actuator.Delete(ctx, bb); err != nil {
		_ = r.statusUpdater.Error(ctx, bb, extensionscontroller.ReconcileErrCauseOrErr(err), operationType, "Error deleting backupbucket")
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, bb, operationType, "Successfully deleted backupbucket"); err != nil {
		return reconcile.Result{}, err
	}

	secret, err := kutil.GetSecretByReference(ctx, r.client, &bb.Spec.SecretRef)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get backup bucket secret: %+v", err)
	}
	if err := controllerutils.RemoveFinalizer(ctx, r.reader, r.client, secret, FinalizerName); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to remove finalizer on bucket secret: %+v", err)
	}

	r.logger.Info("Removing finalizer", "backupbucket", kutil.ObjectName(bb))
	if err := controllerutils.RemoveFinalizer(ctx, r.reader, r.client, bb, FinalizerName); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizer from backupbucket: %+v", err)
	}

	return reconcile.Result{}, nil
}
