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

package controlplane

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
)

const (
	// EventControlPlaneReconciliation an event reason to describe control plane reconciliation.
	EventControlPlaneReconciliation string = "ControlPlaneReconciliation"
	// EventControlPlaneRestoration an event reason to describe control plane restoration.
	EventControlPlaneRestoration string = "ControlPlaneRestoration"
	// EventControlPlaneDeletion an event reason to describe control plane deletion.
	EventControlPlaneDeletion string = "ControlPlaneDeletion"
	// EventControlPlaneMigration an event reason to describe control plane migration.
	EventControlPlaneMigration string = "ControlPlaneMigration"

	// RequeueAfter is the duration to requeue a controlplane reconciliation if indicated by the actuator.
	RequeueAfter = 2 * time.Second
)

type reconciler struct {
	logger   logr.Logger
	actuator Actuator

	client   client.Client
	reader   client.Reader
	recorder record.EventRecorder
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// controlplane resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(mgr manager.Manager, actuator Actuator) reconcile.Reconciler {
	return extensionscontroller.OperationAnnotationWrapper(
		func() client.Object { return &extensionsv1alpha1.ControlPlane{} },
		&reconciler{
			logger:   log.Log.WithName(ControllerName),
			actuator: actuator,
			recorder: mgr.GetEventRecorderFor(ControllerName),
		})
}

func (r *reconciler) InjectFunc(f inject.Func) error {
	return f(r.actuator)
}

func (r *reconciler) InjectClient(client client.Client) error {
	r.client = client
	return nil
}

func (r *reconciler) InjectAPIReader(reader client.Reader) error {
	r.reader = reader
	return nil
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	cp := &extensionsv1alpha1.ControlPlane{}
	if err := r.client.Get(ctx, request.NamespacedName, cp); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	cluster, err := extensionscontroller.GetCluster(ctx, r.client, cp.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}

	if extensionscontroller.IsFailed(cluster) {
		r.logger.Info("Stop reconciling ControlPlane of failed Shoot.", "namespace", request.Namespace, "name", cp.Name)
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(cp.ObjectMeta, cp.Status.LastOperation)

	switch {
	case extensionscontroller.IsMigrated(cp):
		return reconcile.Result{}, nil
	case operationType == gardencorev1beta1.LastOperationTypeMigrate:
		return r.migrate(ctx, cp, cluster)
	case cp.DeletionTimestamp != nil:
		return r.delete(ctx, cp, cluster)
	case cp.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRestore:
		return r.restore(ctx, cp, cluster)
	default:
		return r.reconcile(ctx, cp, cluster, operationType)
	}
}

func (r *reconciler) reconcile(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster, operationType gardencorev1beta1.LastOperationType) (reconcile.Result, error) {
	if err := controllerutils.EnsureFinalizer(ctx, r.reader, r.client, cp, FinalizerName); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.updateStatusProcessing(ctx, cp, operationType, "Reconciling the controlplane"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the reconciliation of controlplane", "controlplane", cp.Name)
	r.recorder.Event(cp, corev1.EventTypeNormal, EventControlPlaneReconciliation, "Reconciling the controlplane")
	requeue, err := r.actuator.Reconcile(ctx, cp, cluster)
	if err != nil {
		msg := "Error reconciling controlplane"
		_ = r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), cp, operationType, msg)
		return extensionscontroller.ReconcileErr(err)
	}

	msg := "Successfully reconciled controlplane"
	r.logger.Info(msg, "controlplane", cp.Name)
	r.recorder.Event(cp, corev1.EventTypeNormal, EventControlPlaneReconciliation, msg)
	if err := r.updateStatusSuccess(ctx, cp, operationType, msg); err != nil {
		return reconcile.Result{}, err
	}

	if requeue {
		return reconcile.Result{RequeueAfter: RequeueAfter}, nil
	}
	return reconcile.Result{}, nil
}

func (r *reconciler) restore(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if err := controllerutils.EnsureFinalizer(ctx, r.reader, r.client, cp, FinalizerName); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.updateStatusProcessing(ctx, cp, gardencorev1beta1.LastOperationTypeRestore, "Restoring the controlplane"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the restoration of controlplane", "controlplane", cp.Name)
	r.recorder.Event(cp, corev1.EventTypeNormal, EventControlPlaneRestoration, "Restoring the controlplane")
	requeue, err := r.actuator.Restore(ctx, cp, cluster)
	if err != nil {
		msg := "Error restoring controlplane"
		_ = r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), cp, gardencorev1beta1.LastOperationTypeRestore, msg)
		return extensionscontroller.ReconcileErr(err)
	}

	msg := "Successfully restored controlplane"
	r.logger.Info(msg, "controlplane", cp.Name)
	r.recorder.Event(cp, corev1.EventTypeNormal, EventControlPlaneRestoration, msg)
	if err := r.updateStatusSuccess(ctx, cp, gardencorev1beta1.LastOperationTypeRestore, msg); err != nil {
		return reconcile.Result{}, err
	}

	if requeue {
		return reconcile.Result{RequeueAfter: RequeueAfter}, nil
	}

	// remove operation annotation 'restore'
	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, cp, v1beta1constants.GardenerOperation); err != nil {
		msg := "Error removing annotation from ControlPlane"
		r.recorder.Eventf(cp, corev1.EventTypeWarning, EventControlPlaneMigration, "%s: %+v", msg, err)
		return reconcile.Result{}, fmt.Errorf("%s: %+v", msg, err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) migrate(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if err := r.updateStatusProcessing(ctx, cp, gardencorev1beta1.LastOperationTypeMigrate, "Migrating the controlplane"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the migration of controlplane", "controlplane", cp.Name)
	r.recorder.Event(cp, corev1.EventTypeNormal, EventControlPlaneMigration, "Migrating the cp")
	if err := r.actuator.Migrate(ctx, cp, cluster); err != nil {
		msg := "Error migrating controlplane"
		r.recorder.Eventf(cp, corev1.EventTypeWarning, EventControlPlaneMigration, "%s: %+v", msg, err)
		_ = r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), cp, gardencorev1beta1.LastOperationTypeMigrate, msg)
		return extensionscontroller.ReconcileErr(err)
	}

	msg := "Successfully migrated controlplane"
	r.logger.Info(msg, "controlplane", cp.Name)
	r.recorder.Event(cp, corev1.EventTypeNormal, EventControlPlaneMigration, msg)
	if err := r.updateStatusSuccess(ctx, cp, gardencorev1beta1.LastOperationTypeMigrate, msg); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Removing all finalizer.", "controlplane", cp.Name)
	if err := extensionscontroller.DeleteAllFinalizers(ctx, r.client, cp); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizers from ControlPlane: %+v", err)
	}

	// remove operation annotation 'migrate'
	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, cp, v1beta1constants.GardenerOperation); err != nil {
		msg := "Error removing annotation from ControlPlane"
		r.recorder.Eventf(cp, corev1.EventTypeWarning, EventControlPlaneMigration, "%s: %+v", msg, err)
		return reconcile.Result{}, fmt.Errorf("%s: %+v", msg, err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(cp, FinalizerName) {
		r.logger.Info("Deleting controlplane causes a no-op as there is no finalizer.", "controlplane", cp.Name)
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(cp.ObjectMeta, cp.Status.LastOperation)
	if err := r.updateStatusProcessing(ctx, cp, operationType, "Deleting the controlplane"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the deletion of controlplane", "controlplane", cp.Name)
	r.recorder.Event(cp, corev1.EventTypeNormal, EventControlPlaneDeletion, "Deleting the cp")
	if err := r.actuator.Delete(ctx, cp, cluster); err != nil {
		msg := "Error deleting controlplane"
		r.recorder.Eventf(cp, corev1.EventTypeWarning, EventControlPlaneDeletion, "%s: %+v", msg, err)
		_ = r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), cp, operationType, msg)
		return extensionscontroller.ReconcileErr(err)
	}

	msg := "Successfully deleted controlplane"
	r.logger.Info(msg, "controlplane", cp.Name)
	r.recorder.Event(cp, corev1.EventTypeNormal, EventControlPlaneDeletion, msg)
	if err := r.updateStatusSuccess(ctx, cp, operationType, msg); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Removing finalizer.", "controlplane", cp.Name)
	if err := controllerutils.RemoveFinalizer(ctx, r.reader, r.client, cp, FinalizerName); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizer from ControlPlane: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) updateStatusProcessing(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, lastOperationType gardencorev1beta1.LastOperationType, description string) error {
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, cp, func() error {
		cp.Status.LastOperation = extensionscontroller.LastOperation(lastOperationType, gardencorev1beta1.LastOperationStateProcessing, 1, description)
		return nil
	})
}

func (r *reconciler) updateStatusError(ctx context.Context, err error, cp *extensionsv1alpha1.ControlPlane, lastOperationType gardencorev1beta1.LastOperationType, description string) error {
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, cp, func() error {
		cp.Status.ObservedGeneration = cp.Generation
		cp.Status.LastOperation, cp.Status.LastError = extensionscontroller.ReconcileError(lastOperationType, gardencorev1beta1helper.FormatLastErrDescription(fmt.Errorf("%s: %v", description, err)), 50, gardencorev1beta1helper.ExtractErrorCodes(gardencorev1beta1helper.DetermineError(err, err.Error()))...)
		return nil
	})
}

func (r *reconciler) updateStatusSuccess(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, lastOperationType gardencorev1beta1.LastOperationType, description string) error {
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, cp, func() error {
		cp.Status.ObservedGeneration = cp.Generation
		cp.Status.LastOperation, cp.Status.LastError = extensionscontroller.ReconcileSucceeded(lastOperationType, description)
		return nil
	})
}
