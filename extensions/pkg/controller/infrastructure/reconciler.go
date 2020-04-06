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

package infrastructure

import (
	"context"
	"fmt"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

const (
	// EventInfrastructureReconciliation an event reason to describe infrastructure reconciliation.
	EventInfrastructureReconciliation string = "InfrastructureReconciliation"
	// EventInfrastructureDeleton an event reason to describe infrastructure deletion.
	EventInfrastructureDeleton string = "InfrastructureDeleton"
)

type reconciler struct {
	logger   logr.Logger
	actuator Actuator

	ctx      context.Context
	client   client.Client
	recorder record.EventRecorder
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// infrastructure resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(mgr manager.Manager, actuator Actuator) reconcile.Reconciler {
	return extensionscontroller.OperationAnnotationWrapper(
		&extensionsv1alpha1.Infrastructure{},
		&reconciler{
			logger:   log.Log.WithName(ControllerName),
			actuator: actuator,
			recorder: mgr.GetEventRecorderFor(ControllerName),
		},
	)
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
	infrastructure := &extensionsv1alpha1.Infrastructure{}
	if err := r.client.Get(r.ctx, request.NamespacedName, infrastructure); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	cluster, err := extensionscontroller.GetCluster(r.ctx, r.client, infrastructure.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}

	if infrastructure.DeletionTimestamp != nil {
		return r.delete(r.ctx, infrastructure, cluster)
	}
	return r.reconcile(r.ctx, infrastructure, cluster)
}

func (r *reconciler) reconcile(ctx context.Context, infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if err := extensionscontroller.EnsureFinalizer(ctx, r.client, FinalizerName, infrastructure); err != nil {
		return reconcile.Result{}, err
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(infrastructure.ObjectMeta, infrastructure.Status.LastOperation)
	if err := r.updateStatusProcessing(ctx, infrastructure, operationType, "Reconciling the infrastructure"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the reconciliation of infrastructure", "infrastructure", infrastructure.Name)
	r.recorder.Event(infrastructure, corev1.EventTypeNormal, EventInfrastructureReconciliation, "Reconciling the infrastructure")
	if err := r.actuator.Reconcile(ctx, infrastructure, cluster); err != nil {
		msg := "Error reconciling infrastructure"
		utilruntime.HandleError(r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), infrastructure, operationType, msg))
		r.logger.Error(err, msg, "infrastructure", infrastructure.Name)
		return extensionscontroller.ReconcileErr(err)
	}

	msg := "Successfully reconciled infrastructure"
	r.logger.Info(msg, "infrastructure", infrastructure.Name)
	r.recorder.Event(infrastructure, corev1.EventTypeNormal, EventInfrastructureReconciliation, msg)
	if err := r.updateStatusSuccess(ctx, infrastructure, operationType, msg); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	hasFinalizer, err := extensionscontroller.HasFinalizer(infrastructure, FinalizerName)
	if err != nil {
		r.logger.Error(err, "Could not instantiate finalizer deletion")
		return reconcile.Result{}, err
	}
	if !hasFinalizer {
		r.logger.Info("Deleting infrastructure causes a no-op as there is no finalizer.", "infrastructure", infrastructure.Name)
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(infrastructure.ObjectMeta, infrastructure.Status.LastOperation)
	if err := r.updateStatusProcessing(ctx, infrastructure, operationType, "Deleting the infrastructure"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the deletion of infrastructure", "infrastructure", infrastructure.Name)
	r.recorder.Event(infrastructure, corev1.EventTypeNormal, EventInfrastructureDeleton, "Deleting the infrastructure")
	if err := r.actuator.Delete(r.ctx, infrastructure, cluster); err != nil {
		msg := "Error deleting infrastructure"
		r.recorder.Eventf(infrastructure, corev1.EventTypeWarning, EventInfrastructureDeleton, "%s: %+v", msg, err)
		utilruntime.HandleError(r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), infrastructure, operationType, msg))
		r.logger.Error(err, msg, "infrastructure", infrastructure.Name)
		return extensionscontroller.ReconcileErr(err)
	}

	msg := "Successfully deleted infrastructure"
	r.logger.Info(msg, "infrastructure", infrastructure.Name)
	r.recorder.Event(infrastructure, corev1.EventTypeNormal, EventInfrastructureDeleton, msg)
	if err := r.updateStatusSuccess(ctx, infrastructure, operationType, msg); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Removing finalizer.", "infrastructure", infrastructure.Name)
	if err := extensionscontroller.DeleteFinalizer(ctx, r.client, FinalizerName, infrastructure); err != nil {
		r.logger.Error(err, "Error removing finalizer from Infrastructure", "infrastructure", infrastructure.Name)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) updateStatusProcessing(ctx context.Context, infrastructure *extensionsv1alpha1.Infrastructure, lastOperationType gardencorev1beta1.LastOperationType, description string) error {
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, infrastructure, func() error {
		infrastructure.Status.LastOperation = extensionscontroller.LastOperation(lastOperationType, gardencorev1beta1.LastOperationStateProcessing, 1, description)
		return nil
	})
}

func (r *reconciler) updateStatusError(ctx context.Context, err error, infrastructure *extensionsv1alpha1.Infrastructure, lastOperationType gardencorev1beta1.LastOperationType, description string) error {
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, infrastructure, func() error {
		infrastructure.Status.ObservedGeneration = infrastructure.Generation
		infrastructure.Status.LastOperation, infrastructure.Status.LastError = extensionscontroller.ReconcileError(lastOperationType, gardencorev1beta1helper.FormatLastErrDescription(fmt.Errorf("%s: %v", description, err)), 50, gardencorev1beta1helper.ExtractErrorCodes(err)...)
		return nil
	})
}

func (r *reconciler) updateStatusSuccess(ctx context.Context, infrastructure *extensionsv1alpha1.Infrastructure, lastOperationType gardencorev1beta1.LastOperationType, description string) error {
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, infrastructure, func() error {
		infrastructure.Status.ObservedGeneration = infrastructure.Generation
		infrastructure.Status.LastOperation, infrastructure.Status.LastError = extensionscontroller.ReconcileSucceeded(lastOperationType, description)
		return nil
	})
}
