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

package extension

import (
	"context"
	"fmt"
	"time"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionshandler "github.com/gardener/gardener/extensions/pkg/handler"
	extensionspredicate "github.com/gardener/gardener/extensions/pkg/predicate"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	contextutil "github.com/gardener/gardener/pkg/utils/context"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// FinalizerPrefix is the prefix name of the finalizer written by this controller.
	FinalizerPrefix = "extensions.gardener.cloud"
)

// AddArgs are arguments for adding an Extension resources controller to a manager.
type AddArgs struct {
	// Actuator is an Extension resource actuator.
	Actuator Actuator
	// Name is the name of the controller.
	Name string
	// FinalizerSuffix is the suffix for the finalizer name.
	FinalizerSuffix string
	// ControllerOptions are the controller options used for creating a controller.
	// The options.Reconciler is always overridden with a reconciler created from the
	// given actuator.
	ControllerOptions controller.Options
	// Predicates are the predicates to use.
	Predicates []predicate.Predicate
	// Resync determines the requeue interval.
	Resync time.Duration
	// Type is the type of the resource considered for reconciliation.
	Type string
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation or not.
	// If the annotation is not ignored, the extension controller will only reconcile
	// with a present operation annotation typically set during a reconcile (e.g in the maintenance time) by the Gardenlet
	IgnoreOperationAnnotation bool
}

// Add adds an Extension controller to the given manager using the given AddArgs.
func Add(mgr manager.Manager, args AddArgs) error {
	args.ControllerOptions.Reconciler = NewReconciler(args)
	return add(mgr, args)
}

// DefaultPredicates returns the default predicates for an extension reconciler.
func DefaultPredicates(ignoreOperationAnnotation bool) []predicate.Predicate {
	if ignoreOperationAnnotation {
		return []predicate.Predicate{
			predicate.GenerationChangedPredicate{},
		}
	}
	return []predicate.Predicate{
		predicate.Or(
			extensionspredicate.HasOperationAnnotation(),
			extensionspredicate.LastOperationNotSuccessful(),
			extensionspredicate.IsDeleting(),
		),
		extensionspredicate.ShootNotFailed(),
	}
}

func add(mgr manager.Manager, args AddArgs) error {
	ctrl, err := controller.New(args.Name, mgr, args.ControllerOptions)
	if err != nil {
		return err
	}

	predicates := extensionspredicate.AddTypePredicate(args.Predicates, args.Type)

	if args.IgnoreOperationAnnotation {
		if err := ctrl.Watch(&source.Kind{Type: &extensionsv1alpha1.Cluster{}}, &extensionshandler.EnqueueRequestsFromMapFunc{
			ToRequests: extensionshandler.SimpleMapper(ClusterToExtensionMapper(predicates...), extensionshandler.UpdateWithNew),
		}); err != nil {
			return err
		}
	}

	return ctrl.Watch(&source.Kind{Type: &extensionsv1alpha1.Extension{}}, &handler.EnqueueRequestForObject{}, predicates...)
}

// reconciler reconciles Extension resources of Gardener's
// `extensions.gardener.cloud` API group.
type reconciler struct {
	logger        logr.Logger
	actuator      Actuator
	finalizerName string

	ctx    context.Context
	client client.Client

	resync time.Duration
}

var _ reconcile.Reconciler = (*reconciler)(nil)

// NewReconciler creates a new reconcile.Reconciler that reconciles
// Extension resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(args AddArgs) reconcile.Reconciler {
	logger := log.Log.WithName(args.Name)
	finalizer := fmt.Sprintf("%s/%s", FinalizerPrefix, args.FinalizerSuffix)
	return extensionscontroller.OperationAnnotationWrapper(
		&extensionsv1alpha1.Extension{},
		&reconciler{
			logger:        logger,
			actuator:      args.Actuator,
			finalizerName: finalizer,
			resync:        args.Resync,
		})
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
	r.ctx = contextutil.FromStopChannel(stopCh)
	return nil
}

// Reconcile is the reconciler function that gets executed in case there are new events for `Extension` resources.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ex := &extensionsv1alpha1.Extension{}
	if err := r.client.Get(r.ctx, request.NamespacedName, ex); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("could not fetch Extension resource: %+v", err)
	}

	var result reconcile.Result

	shoot, err := extensionscontroller.GetShoot(r.ctx, r.client, request.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}

	if extensionscontroller.IsShootFailed(shoot) {
		r.logger.Info("Stop reconciling Extension of failed Shoot.", "namespace", request.Namespace, "name", ex.Name)
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(ex.ObjectMeta, ex.Status.LastOperation)

	switch {
	case extensionscontroller.IsMigrated(ex):
		return reconcile.Result{}, nil
	case operationType == gardencorev1beta1.LastOperationTypeMigrate:
		return r.migrate(r.ctx, ex)
	case ex.DeletionTimestamp != nil:
		return r.delete(r.ctx, ex)
	case ex.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRestore:
		return r.restore(r.ctx, ex, operationType)
	default:
		if result, err = r.reconcile(r.ctx, ex, operationType); err != nil {
			return result, err
		}
		return reconcile.Result{Requeue: r.resync != 0, RequeueAfter: r.resync}, nil
	}
}

func (r *reconciler) reconcile(ctx context.Context, ex *extensionsv1alpha1.Extension, operationType gardencorev1beta1.LastOperationType) (reconcile.Result, error) {
	if err := extensionscontroller.EnsureFinalizer(ctx, r.client, r.finalizerName, ex); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.updateStatusProcessing(ctx, ex, operationType, "Reconciling Extension resource"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Reconcile(ctx, ex); err != nil {
		_ = r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), ex, operationType, "Unable to reconcile Extension resource")
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.updateStatusSuccess(ctx, ex, operationType, "Successfully reconciled Extension resource"); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, ex *extensionsv1alpha1.Extension) (reconcile.Result, error) {
	hasFinalizer, err := extensionscontroller.HasFinalizer(ex, r.finalizerName)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not instantiate finalizer deletion: %+v", err)
	}

	if !hasFinalizer {
		r.logger.Info("Reconciling Extension resource causes a no-op as there is no finalizer.", "extension", ex.Name, "namespace", ex.Namespace)
		return reconcile.Result{}, nil
	}

	if err := r.updateStatusProcessing(ctx, ex, gardencorev1beta1.LastOperationTypeDelete, "Deleting Extension resource."); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Delete(ctx, ex); err != nil {
		_ = r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), ex, gardencorev1beta1.LastOperationTypeDelete, "Error deleting Extension resource")
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.updateStatusSuccess(ctx, ex, gardencorev1beta1.LastOperationTypeDelete, "Successfully deleted Extension resource"); err != nil {
		return reconcile.Result{}, err
	}

	if err := extensionscontroller.DeleteFinalizer(ctx, r.client, r.finalizerName, ex); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizer from Extension resource: %+v", err)
	}
	return reconcile.Result{}, nil
}

func (r *reconciler) restore(ctx context.Context, ex *extensionsv1alpha1.Extension, operationType gardencorev1beta1.LastOperationType) (reconcile.Result, error) {
	if err := extensionscontroller.EnsureFinalizer(ctx, r.client, r.finalizerName, ex); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.updateStatusProcessing(ctx, ex, operationType, "Restoring Extension resource"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Restore(ctx, ex); err != nil {
		_ = r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), ex, operationType, "Unable to restore Extension resource")
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.updateStatusSuccess(ctx, ex, operationType, "Successfully restored Extension resource"); err != nil {
		return reconcile.Result{}, err
	}

	// remove operation annotation 'restore'
	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, ex, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from Extension resource: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) migrate(ctx context.Context, ex *extensionsv1alpha1.Extension) (reconcile.Result, error) {
	if err := r.updateStatusProcessing(ctx, ex, gardencorev1beta1.LastOperationTypeMigrate, "Migrate Extension resource."); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Migrate(ctx, ex); err != nil {
		_ = r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), ex, gardencorev1beta1.LastOperationTypeMigrate, "Error migrating Extension resource")
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.updateStatusSuccess(ctx, ex, gardencorev1beta1.LastOperationTypeMigrate, "Successfully migrated Extension resource"); err != nil {
		return reconcile.Result{}, err
	}

	if err := extensionscontroller.DeleteAllFinalizers(ctx, r.client, ex); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing all finalizers from Extension resource: %+v", err)
	}

	// remove operation annotation 'migrate'
	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, ex, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from Extension resource: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) updateStatusProcessing(ctx context.Context, ex *extensionsv1alpha1.Extension, lastOperationType gardencorev1beta1.LastOperationType, description string) error {
	r.logger.Info(description, "extension", ex.Name, "namespace", ex.Namespace)
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, ex, func() error {
		ex.Status.LastOperation = extensionscontroller.LastOperation(lastOperationType, gardencorev1beta1.LastOperationStateProcessing, 1, description)
		return nil
	})
}

func (r *reconciler) updateStatusError(ctx context.Context, err error, ex *extensionsv1alpha1.Extension, lastOperationType gardencorev1beta1.LastOperationType, description string) error {
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, ex, func() error {
		ex.Status.ObservedGeneration = ex.Generation
		ex.Status.LastOperation, ex.Status.LastError = extensionscontroller.ReconcileError(lastOperationType, gardencorev1beta1helper.FormatLastErrDescription(fmt.Errorf("%s: %v", description, err)), 50, gardencorev1beta1helper.ExtractErrorCodes(err)...)
		return nil
	})
}

func (r *reconciler) updateStatusSuccess(ctx context.Context, ex *extensionsv1alpha1.Extension, lastOperationType gardencorev1beta1.LastOperationType, description string) error {
	r.logger.Info(description, "extension", ex.Name, "namespace", ex.Namespace)
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, ex, func() error {
		ex.Status.ObservedGeneration = ex.Generation
		ex.Status.LastOperation, ex.Status.LastError = extensionscontroller.ReconcileSucceeded(lastOperationType, description)
		return nil
	})
}
