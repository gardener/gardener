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

package worker

import (
	"context"
	"fmt"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	contextutil "github.com/gardener/gardener/pkg/utils/context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

const (
	// StartToSyncState is used as part of the Event 'reason' when a Worker state starts to synchronize
	StartToSyncState = "SynchronizingState"
	// SuccessSynced is used as part of the Event 'reason' when a Worker state is synced
	SuccessSynced = "StateSynced"
	// ErrorStateSync is used as part of the Event 'reason' when a Worker state fail to sync
	ErrorStateSync = "ErrorSynchronizingState"
	// StateSyncControllerName is the name of the controller which synchronize the Worker state
	StateSyncControllerName = "worker-state-controller"
)

type stateReconciler struct {
	logger   logr.Logger
	actuator StateActuator
	recorder record.EventRecorder

	ctx    context.Context
	client client.Client
}

// NewStateReconciler creates a new reconcile.Reconciler that reconciles
// Worker's State resources of Gardener's `extensions.gardener.cloud` API group.
func NewStateReconciler(mgr manager.Manager, actuator StateActuator) reconcile.Reconciler {
	return &stateReconciler{
		logger:   log.Log.WithName(StateUpdatingControllerName),
		actuator: actuator,
		recorder: mgr.GetEventRecorderFor(StateSyncControllerName),
	}
}

func (r *stateReconciler) InjectFunc(f inject.Func) error {
	return f(r.actuator)
}

func (r *stateReconciler) InjectClient(client client.Client) error {
	r.client = client
	return nil
}

func (r *stateReconciler) InjectStopChannel(stopCh <-chan struct{}) error {
	r.ctx = contextutil.FromStopChannel(stopCh)
	return nil
}

func (r *stateReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	worker := &extensionsv1alpha1.Worker{}
	if err := r.client.Get(r.ctx, request.NamespacedName, worker); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Deletion flow
	if worker.DeletionTimestamp != nil {
		//Nothing to do
		return reconcile.Result{}, nil
	}

	// Reconcile flow
	operationType := gardencorev1beta1helper.ComputeOperationType(worker.ObjectMeta, worker.Status.LastOperation)
	if operationType != gardencorev1beta1.LastOperationTypeReconcile {
		return reconcile.Result{Requeue: true}, nil
	} else if isWorkerMigrated(worker) {
		//Nothing to do
		return reconcile.Result{}, nil
	}

	r.recorder.Event(worker, corev1.EventTypeNormal, StartToSyncState, "Updating the worker state")

	if err := r.actuator.Reconcile(r.ctx, worker); err != nil {
		msg := "Error updating worker state"
		r.logger.Error(err, msg, "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
		r.recorder.Event(worker, corev1.EventTypeWarning, ErrorStateSync, msg)
		return extensionscontroller.ReconcileErr(err)
	}

	msg := "Successfully updated worker state"
	r.logger.Info(msg, "worker", fmt.Sprintf("%s/%s", worker.Namespace, worker.Name))
	r.recorder.Event(worker, corev1.EventTypeNormal, SuccessSynced, msg)

	return reconcile.Result{}, nil
}
