// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package containerruntime

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

const (
	// EventContainerRuntimeReconciliation an event reason to describe container runtime reconciliation.
	EventContainerRuntimeReconciliation string = "ContainerRuntimeReconciliation"
	// EventContainerRuntimeDeletion an event reason to describe container runtime deletion.
	EventContainerRuntimeDeletion string = "ContainerRuntimeDeletion"
	// EventContainerRuntimeRestoration an event reason to describe container runtime restoration.
	EventContainerRuntimeRestoration string = "ContainerRuntimeRestoration"
	// EventContainerRuntimeMigration an event reason to describe container runtime migration.
	EventContainerRuntimeMigration string = "ContainerRuntimeMigration"
)

// reconciler reconciles ContainerRuntime resources of Gardener's
// `extensions.gardener.cloud` API group.
type reconciler struct {
	logger   logr.Logger
	actuator Actuator

	ctx      context.Context
	client   client.Client
	recorder record.EventRecorder
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// ContainerRuntime resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(mgr manager.Manager, actuator Actuator) reconcile.Reconciler {
	return extensionscontroller.OperationAnnotationWrapper(
		&extensionsv1alpha1.ContainerRuntime{},
		&reconciler{
			logger:   log.Log.WithName(ControllerName),
			actuator: actuator,
			recorder: mgr.GetEventRecorderFor(ControllerName),
		},
	)
}

// InjectFunc enables dependency injection into the actuator.
func (r *reconciler) InjectFunc(f inject.Func) error {
	return f(r.actuator)
}

// InjectClient injects the controller runtime client into the reconciler.
func (r *reconciler) InjectClient(client client.Client) error {
	r.client = client
	return nil
}

// InjectStopChannel is an implementation for getting the respective stop channel managed by the controller-runtime.
func (r *reconciler) InjectStopChannel(stopCh <-chan struct{}) error {
	r.ctx = util.ContextFromStopChannel(stopCh)
	return nil
}

// Reconcile is the reconciler function that gets executed in case there are new events for `ContainerRuntime` resources.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	cr := &extensionsv1alpha1.ContainerRuntime{}
	if err := r.client.Get(r.ctx, request.NamespacedName, cr); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	cluster, err := extensionscontroller.GetCluster(r.ctx, r.client, cr.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}
	if extensionscontroller.IsFailed(cluster) {
		r.logger.Info("Stop reconciling ContainerRuntime of failed Shoot.", "namespace", request.Namespace, "name", cr.Name)
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(cr.ObjectMeta, cr.Status.LastOperation)

	switch {
	case extensionscontroller.IsMigrated(cr):
		return reconcile.Result{}, nil
	case operationType == gardencorev1beta1.LastOperationTypeMigrate:
		return r.migrate(r.ctx, cr, cluster)
	case cr.DeletionTimestamp != nil:
		return r.delete(r.ctx, cr, cluster)
	case cr.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRestore:
		return r.restore(r.ctx, cr, cluster, operationType)
	default:
		return r.reconcile(r.ctx, cr, cluster, operationType)
	}
}

func (r *reconciler) reconcile(ctx context.Context, cr *extensionsv1alpha1.ContainerRuntime, cluster *extensionscontroller.Cluster, operationType gardencorev1beta1.LastOperationType) (reconcile.Result, error) {
	if err := extensionscontroller.EnsureFinalizer(ctx, r.client, FinalizerName, cr); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.updateStatusProcessing(ctx, cr, operationType, EventContainerRuntimeReconciliation, "Reconciling the container runtime"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Reconcile(ctx, cr, cluster); err != nil {
		utilruntime.HandleError(r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), cr, operationType, EventContainerRuntimeReconciliation, "Error reconciling container runtime"))
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.updateStatusSuccess(ctx, cr, operationType, EventContainerRuntimeReconciliation, "Successfully reconciled container runtime"); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *reconciler) restore(ctx context.Context, cr *extensionsv1alpha1.ContainerRuntime, cluster *extensionscontroller.Cluster, operationType gardencorev1beta1.LastOperationType) (reconcile.Result, error) {
	if err := extensionscontroller.EnsureFinalizer(ctx, r.client, FinalizerName, cr); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.updateStatusProcessing(ctx, cr, operationType, EventContainerRuntimeRestoration, "Restoring the container runtime"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Restore(ctx, cr, cluster); err != nil {
		utilruntime.HandleError(r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), cr, operationType, EventContainerRuntimeRestoration, "Error restoring container runtime"))
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.updateStatusSuccess(ctx, cr, operationType, EventContainerRuntimeRestoration, "Successfully restored container runtime"); err != nil {
		return reconcile.Result{}, err
	}

	// remove operation annotation 'restore'
	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, cr, v1beta1constants.GardenerOperation); err != nil {
		msg := "Error removing annotation from ContainerRuntime"
		r.recorder.Eventf(cr, corev1.EventTypeWarning, EventContainerRuntimeRestoration, "%s: %+v", msg, err)
		return reconcile.Result{}, fmt.Errorf("%s: %+v", msg, err)
	}
	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, cr *extensionsv1alpha1.ContainerRuntime, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	hasFinalizer, err := extensionscontroller.HasFinalizer(cr, FinalizerName)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not instantiate finalizer deletion: %+v", err)
	}
	if !hasFinalizer {
		r.logger.Info("Deleting container runtime causes a no-op as there is no finalizer.", "containerruntime", cr.Name)
		return reconcile.Result{}, nil
	}

	if err := r.updateStatusProcessing(ctx, cr, gardencorev1beta1.LastOperationTypeDelete, EventContainerRuntimeDeletion, "Deleting the container runtime"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Delete(r.ctx, cr, cluster); err != nil {
		utilruntime.HandleError(r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), cr, gardencorev1beta1.LastOperationTypeDelete, EventContainerRuntimeDeletion, "Error deleting container runtime"))
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.updateStatusSuccess(ctx, cr, gardencorev1beta1.LastOperationTypeDelete, EventContainerRuntimeDeletion, "Successfully deleted container runtime"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Removing finalizer.", "containerruntime", cr.Name)
	if err := extensionscontroller.DeleteFinalizer(ctx, r.client, FinalizerName, cr); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizer from the container runtime resource: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) migrate(ctx context.Context, cr *extensionsv1alpha1.ContainerRuntime, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if err := r.updateStatusProcessing(ctx, cr, gardencorev1beta1.LastOperationTypeMigrate, EventContainerRuntimeMigration, "Migrating the container runtime"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Migrate(r.ctx, cr, cluster); err != nil {
		utilruntime.HandleError(r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), cr, gardencorev1beta1.LastOperationTypeMigrate, EventContainerRuntimeMigration, "Error migrating container runtime"))
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.updateStatusSuccess(ctx, cr, gardencorev1beta1.LastOperationTypeMigrate, EventContainerRuntimeMigration, "Successfully migrated container runtime"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Removing all finalizers.", "containerruntime", cr.Name)
	if err := extensionscontroller.DeleteAllFinalizers(ctx, r.client, cr); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizers from the container runtime resource: %+v", err)
	}

	// remove operation annotation 'migrate'
	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, cr, v1beta1constants.GardenerOperation); err != nil {
		msg := "Error removing annotation from ContainerRuntime"
		r.recorder.Eventf(cr, corev1.EventTypeWarning, EventContainerRuntimeMigration, "%s: %+v", msg, err)
		return reconcile.Result{}, fmt.Errorf("%s: %+v", msg, err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) updateStatusProcessing(ctx context.Context, cr *extensionsv1alpha1.ContainerRuntime, lastOperationType gardencorev1beta1.LastOperationType, eventReason, description string) error {
	r.logger.Info(description, "containerruntime", cr.Name)
	r.recorder.Event(cr, corev1.EventTypeNormal, eventReason, description)
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, cr, func() error {
		cr.Status.LastOperation = extensionscontroller.LastOperation(lastOperationType, gardencorev1beta1.LastOperationStateProcessing, 1, description)
		return nil
	})
}

func (r *reconciler) updateStatusError(ctx context.Context, err error, cr *extensionsv1alpha1.ContainerRuntime, lastOperationType gardencorev1beta1.LastOperationType, eventReason, description string) error {
	r.recorder.Eventf(cr, corev1.EventTypeWarning, eventReason, "%s: %+v", description, err)
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, cr, func() error {
		cr.Status.ObservedGeneration = cr.Generation
		cr.Status.LastOperation, cr.Status.LastError = extensionscontroller.ReconcileError(lastOperationType, gardencorev1beta1helper.FormatLastErrDescription(fmt.Errorf("%s: %v", description, err)), 50, gardencorev1beta1helper.ExtractErrorCodes(err)...)
		return nil
	})
}

func (r *reconciler) updateStatusSuccess(ctx context.Context, cr *extensionsv1alpha1.ContainerRuntime, lastOperationType gardencorev1beta1.LastOperationType, eventReason, description string) error {
	r.logger.Info(description, "containerruntime", cr.Name)
	r.recorder.Event(cr, corev1.EventTypeNormal, eventReason, description)
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, cr, func() error {
		cr.Status.ObservedGeneration = cr.Generation
		cr.Status.LastOperation, cr.Status.LastError = extensionscontroller.ReconcileSucceeded(lastOperationType, description)
		return nil
	})
}
