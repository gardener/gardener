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

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
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
	// EventControlPlaneReconciliation an event reason to describe control plane reconciliation.
	EventControlPlaneReconciliation string = "ControlPlaneReconciliation"
	// EventControlPlaneDeletion an event reason to describe control plane deletion.
	EventControlPlaneDeletion string = "ControlPlaneDeletion"

	// RequeueAfter is the duration to requeue a controlplane reconciliation if indicated by the actuator.
	RequeueAfter time.Duration = 2 * time.Second
)

type reconciler struct {
	logger   logr.Logger
	actuator Actuator

	ctx      context.Context
	client   client.Client
	recorder record.EventRecorder
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// controlplane resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(mgr manager.Manager, actuator Actuator) reconcile.Reconciler {
	return extensionscontroller.OperationAnnotationWrapper(
		&extensionsv1alpha1.ControlPlane{},
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

func (r *reconciler) InjectStopChannel(stopCh <-chan struct{}) error {
	r.ctx = util.ContextFromStopChannel(stopCh)
	return nil
}

func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	cp := &extensionsv1alpha1.ControlPlane{}
	if err := r.client.Get(r.ctx, request.NamespacedName, cp); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	cluster, err := extensionscontroller.GetCluster(r.ctx, r.client, cp.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}

	if cp.DeletionTimestamp != nil {
		return r.delete(r.ctx, cp, cluster)
	}
	return r.reconcile(r.ctx, cp, cluster)
}

func (r *reconciler) reconcile(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if err := extensionscontroller.EnsureFinalizer(ctx, r.client, FinalizerName, cp); err != nil {
		return reconcile.Result{}, err
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(cp.ObjectMeta, cp.Status.LastOperation)
	if err := r.updateStatusProcessing(ctx, cp, operationType, "Reconciling the controlplane"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the reconciliation of controlplane", "controlplane", cp.Name)
	r.recorder.Event(cp, corev1.EventTypeNormal, EventControlPlaneReconciliation, "Reconciling the controlplane")
	requeue, err := r.actuator.Reconcile(ctx, cp, cluster)
	if err != nil {
		msg := "Error reconciling controlplane"
		_ = r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), cp, operationType, msg)
		r.logger.Error(err, msg, "controlplane", cp.Name)
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

func (r *reconciler) delete(ctx context.Context, cp *extensionsv1alpha1.ControlPlane, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	hasFinalizer, err := extensionscontroller.HasFinalizer(cp, FinalizerName)
	if err != nil {
		r.logger.Error(err, "Could not instantiate finalizer deletion")
		return reconcile.Result{}, err
	}
	if !hasFinalizer {
		r.logger.Info("Deleting controlplane causes a no-op as there is no finalizer.", "controlplane", cp.Name)
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(cp.ObjectMeta, cp.Status.LastOperation)
	if err := r.updateStatusProcessing(ctx, cp, operationType, "Deleting the controlplane"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the deletion of controlplane", "controlplane", cp.Name)
	r.recorder.Event(cp, corev1.EventTypeNormal, EventControlPlaneDeletion, "Deleting the cp")
	if err := r.actuator.Delete(r.ctx, cp, cluster); err != nil {
		msg := "Error deleting controlplane"
		r.recorder.Eventf(cp, corev1.EventTypeWarning, EventControlPlaneDeletion, "%s: %+v", msg, err)
		_ = r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), cp, operationType, msg)
		r.logger.Error(err, msg, "controlplane", cp.Name)
		return extensionscontroller.ReconcileErr(err)
	}

	msg := "Successfully deleted controlplane"
	r.logger.Info(msg, "controlplane", cp.Name)
	r.recorder.Event(cp, corev1.EventTypeNormal, EventControlPlaneDeletion, msg)
	if err := r.updateStatusSuccess(ctx, cp, operationType, msg); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Removing finalizer.", "controlplane", cp.Name)
	if err := extensionscontroller.DeleteFinalizer(ctx, r.client, FinalizerName, cp); err != nil {
		r.logger.Error(err, "Error removing finalizer from ControlPlane", "controlplane", cp.Name)
		return reconcile.Result{}, err
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
		cp.Status.LastOperation, cp.Status.LastError = extensionscontroller.ReconcileError(lastOperationType, gardencorev1beta1helper.FormatLastErrDescription(fmt.Errorf("%s: %v", description, err)), 50, gardencorev1beta1helper.ExtractErrorCodes(err)...)
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
