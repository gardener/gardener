// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// TODO(rfranzke): Drop this stateReconciler after a few releases as soon as the shoot migrate flow persists the Shoot
//  state only after all extension resources have been migrated.

type stateReconciler struct {
	client client.Client
}

// NewStateReconciler creates a new reconcile.Reconciler that reconciles
// Worker's State resources of Gardener's `extensions.gardener.cloud` API group.
func NewStateReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &stateReconciler{client: mgr.GetClient()}
}

func (r *stateReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	worker := &extensionsv1alpha1.Worker{}
	if err := r.client.Get(ctx, request.NamespacedName, worker); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if worker.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}

	if v1beta1helper.ComputeOperationType(worker.ObjectMeta, worker.Status.LastOperation) != gardencorev1beta1.LastOperationTypeReconcile {
		return reconcile.Result{Requeue: true}, nil
	}

	if isWorkerMigrated(worker) {
		return reconcile.Result{}, nil
	}

	return reconcile.Result{}, PersistState(ctx, log, r.client, worker)
}

// PersistState persists the worker state into the .status.state field.
func PersistState(ctx context.Context, log logr.Logger, c client.Client, worker *extensionsv1alpha1.Worker) error {
	// This will be removed in a subsequent commit - just deleted it for better traceability of the changes when reviewing the PR.
	var state *State
	var err error
	if err != nil {
		return err
	}

	rawState, err := json.Marshal(state)
	if err != nil {
		return err
	}

	// If the state did not change, do not even try to send an empty PATCH request.
	if worker.Status.State != nil && bytes.Equal(rawState, worker.Status.State.Raw) {
		return nil
	}

	patch := client.MergeFromWithOptions(worker.DeepCopy(), client.MergeFromWithOptimisticLock{})
	worker.Status.State = &runtime.RawExtension{Raw: rawState}
	if err := c.Status().Patch(ctx, worker, patch); err != nil {
		return fmt.Errorf("error updating Worker state: %w", err)
	}

	log.Info("Successfully updated Worker state")
	return nil
}

func isWorkerMigrated(worker *extensionsv1alpha1.Worker) bool {
	return worker.Status.LastOperation != nil &&
		worker.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeMigrate &&
		worker.Status.LastOperation.State == gardencorev1beta1.LastOperationStateSucceeded
}
