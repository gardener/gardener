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

package bastion

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
)

const (
	// EventBastionReconciliation an event reason to describe bastion reconciliation.
	EventBastionReconciliation string = "BastionReconciliation"
	// EventBastionDeletion an event reason to describe bastion deletion.
	EventBastionDeletion string = "BastionDeletion"
)

type reconciler struct {
	logger   logr.Logger
	actuator Actuator

	client   client.Client
	reader   client.Reader
	recorder record.EventRecorder
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// bastion resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(mgr manager.Manager, actuator Actuator) reconcile.Reconciler {
	return extensionscontroller.OperationAnnotationWrapper(
		func() client.Object { return &extensionsv1alpha1.Bastion{} },
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
	bastion := &extensionsv1alpha1.Bastion{}
	if err := r.client.Get(ctx, request.NamespacedName, bastion); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	cluster, err := extensionscontroller.GetCluster(ctx, r.client, bastion.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}

	if extensionscontroller.IsFailed(cluster) {
		r.logger.Info("Stop reconciling Bastion of failed Shoot.", "name", bastion.Name)
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(bastion.ObjectMeta, bastion.Status.LastOperation)

	switch {
	case bastion.DeletionTimestamp != nil:
		return r.delete(ctx, bastion, cluster)
	default:
		return r.reconcile(ctx, bastion, cluster, operationType)
	}
}

func (r *reconciler) reconcile(ctx context.Context, bastion *extensionsv1alpha1.Bastion, cluster *extensionscontroller.Cluster, operationType gardencorev1beta1.LastOperationType) (reconcile.Result, error) {
	if err := controllerutils.EnsureFinalizer(ctx, r.reader, r.client, bastion, FinalizerName); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.updateStatusProcessing(ctx, bastion, operationType, "Reconciling the bastion"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the reconciliation of bastion", "bastion", bastion.Name)
	r.recorder.Event(bastion, corev1.EventTypeNormal, EventBastionReconciliation, "Reconciling the bastion")
	if err := r.actuator.Reconcile(ctx, bastion, cluster); err != nil {
		msg := "Error reconciling bastion"
		_ = r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), bastion, operationType, msg)
		return extensionscontroller.ReconcileErr(err)
	}

	msg := "Successfully reconciled bastion"
	r.logger.Info(msg, "bastion", bastion.Name)
	r.recorder.Event(bastion, corev1.EventTypeNormal, EventBastionReconciliation, msg)
	if err := r.updateStatusSuccess(ctx, bastion, operationType, msg); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, bastion *extensionsv1alpha1.Bastion, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(bastion, FinalizerName) {
		r.logger.Info("Deleting bastion causes a no-op as there is no finalizer.", "bastion", bastion.Name)
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(bastion.ObjectMeta, bastion.Status.LastOperation)
	if err := r.updateStatusProcessing(ctx, bastion, operationType, "Deleting the bastion"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the deletion of bastion", "bastion", bastion.Name)
	r.recorder.Event(bastion, corev1.EventTypeNormal, EventBastionDeletion, "Deleting the bastion")

	if err := r.actuator.Delete(ctx, bastion, cluster); err != nil {
		msg := "Error deleting bastion"
		r.recorder.Eventf(bastion, corev1.EventTypeWarning, EventBastionDeletion, "%s: %+v", msg, err)
		_ = r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), bastion, operationType, msg)
		return extensionscontroller.ReconcileErr(err)
	}

	msg := "Successfully deleted bastion"
	r.logger.Info(msg, "bastion", bastion.Name)
	r.recorder.Event(bastion, corev1.EventTypeNormal, EventBastionDeletion, msg)
	if err := r.updateStatusSuccess(ctx, bastion, operationType, msg); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Removing finalizer.", "bastion", bastion.Name)
	if err := controllerutils.RemoveFinalizer(ctx, r.reader, r.client, bastion, FinalizerName); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizer from bastion: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) updateStatusProcessing(ctx context.Context, bastion *extensionsv1alpha1.Bastion, lastOperationType gardencorev1beta1.LastOperationType, description string) error {
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, bastion, func() error {
		bastion.Status.LastOperation = extensionscontroller.LastOperation(lastOperationType, gardencorev1beta1.LastOperationStateProcessing, 1, description)
		return nil
	})
}

func (r *reconciler) updateStatusError(ctx context.Context, err error, bastion *extensionsv1alpha1.Bastion, lastOperationType gardencorev1beta1.LastOperationType, description string) error {
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, bastion, func() error {
		bastion.Status.ObservedGeneration = bastion.Generation
		bastion.Status.LastOperation, bastion.Status.LastError = extensionscontroller.ReconcileError(lastOperationType, gardencorev1beta1helper.FormatLastErrDescription(fmt.Errorf("%s: %v", description, err)), 50, gardencorev1beta1helper.ExtractErrorCodes(gardencorev1beta1helper.DetermineError(err, err.Error()))...)
		return nil
	})
}

func (r *reconciler) updateStatusSuccess(ctx context.Context, bastion *extensionsv1alpha1.Bastion, lastOperationType gardencorev1beta1.LastOperationType, description string) error {
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, bastion, func() error {
		bastion.Status.ObservedGeneration = bastion.Generation
		bastion.Status.LastOperation, bastion.Status.LastError = extensionscontroller.ReconcileSucceeded(lastOperationType, description)
		return nil
	})
}
