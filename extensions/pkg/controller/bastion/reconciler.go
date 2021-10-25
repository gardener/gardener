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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
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
// bastion resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(actuator Actuator) reconcile.Reconciler {
	logger := log.Log.WithName(ControllerName)

	return reconcilerutils.OperationAnnotationWrapper(
		func() client.Object { return &extensionsv1alpha1.Bastion{} },
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

	if err := r.statusUpdater.Processing(ctx, bastion, operationType, "Reconciling the bastion"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the reconciliation of bastion", "bastion", kutil.ObjectName(bastion))
	if err := r.actuator.Reconcile(ctx, bastion, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, bastion, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Error reconciling bastion")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, bastion, operationType, "Successfully reconciled bastion"); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, bastion *extensionsv1alpha1.Bastion, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(bastion, FinalizerName) {
		r.logger.Info("Deleting bastion causes a no-op as there is no finalizer", "bastion", kutil.ObjectName(bastion))
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(bastion.ObjectMeta, bastion.Status.LastOperation)
	if err := r.statusUpdater.Processing(ctx, bastion, operationType, "Deleting the bastion"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the deletion of bastion", "bastion", kutil.ObjectName(bastion))

	if err := r.actuator.Delete(ctx, bastion, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, bastion, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Error deleting bastion")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, bastion, operationType, "Successfully reconciled bastion"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Removing finalizer", "bastion", kutil.ObjectName(bastion))
	if err := controllerutils.RemoveFinalizer(ctx, r.reader, r.client, bastion, FinalizerName); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizer from bastion: %+v", err)
	}

	return reconcile.Result{}, nil
}
