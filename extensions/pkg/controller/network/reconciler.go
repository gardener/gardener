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

package network

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
	"github.com/gardener/gardener/extensions/pkg/controller/common"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
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
// Network resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(actuator Actuator) reconcile.Reconciler {
	logger := log.Log.WithName(ControllerName)

	return reconcilerutils.OperationAnnotationWrapper(
		func() client.Object { return &extensionsv1alpha1.Network{} },
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
	network := &extensionsv1alpha1.Network{}
	if err := r.client.Get(ctx, request.NamespacedName, network); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	cluster, err := extensionscontroller.GetCluster(ctx, r.client, network.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}

	logger := r.logger.WithValues("network", kutil.ObjectName(network))
	if extensionscontroller.IsFailed(cluster) {
		logger.Info("Skipping the reconciliation of network of failed shoot")
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(network.ObjectMeta, network.Status.LastOperation)

	if cluster.Shoot != nil && operationType != gardencorev1beta1.LastOperationTypeMigrate {
		key := "network:" + kutil.ObjectName(network)
		ok, watchdogCtx, cleanup, err := common.GetOwnerCheckResultAndContext(ctx, r.client, network.Namespace, cluster.Shoot.Name, key)
		if err != nil {
			return reconcile.Result{}, err
		} else if !ok {
			return reconcile.Result{}, fmt.Errorf("this seed is not the owner of shoot %s", kutil.ObjectName(cluster.Shoot))
		}
		ctx = watchdogCtx
		if cleanup != nil {
			defer cleanup()
		}
	}

	switch {
	case extensionscontroller.ShouldSkipOperation(operationType, network):
		return reconcile.Result{}, nil
	case operationType == gardencorev1beta1.LastOperationTypeMigrate:
		return r.migrate(ctx, network, cluster)
	case network.DeletionTimestamp != nil:
		return r.delete(ctx, network, cluster)
	case operationType == gardencorev1beta1.LastOperationTypeRestore:
		return r.restore(ctx, network, cluster)
	default:
		return r.reconcile(ctx, network, cluster, operationType)
	}
}

func (r *reconciler) reconcile(ctx context.Context, network *extensionsv1alpha1.Network, cluster *extensionscontroller.Cluster, operationType gardencorev1beta1.LastOperationType) (reconcile.Result, error) {
	if err := controllerutils.EnsureFinalizer(ctx, r.reader, r.client, network, FinalizerName); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.statusUpdater.Processing(ctx, network, operationType, "Reconciling the network"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the reconciliation of network", "network", kutil.ObjectName(network))
	if err := r.actuator.Reconcile(ctx, network, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, network, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Error reconciling network")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, network, operationType, "Successfully reconciled network"); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) restore(ctx context.Context, network *extensionsv1alpha1.Network, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if err := controllerutils.EnsureFinalizer(ctx, r.reader, r.client, network, FinalizerName); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.statusUpdater.Processing(ctx, network, gardencorev1beta1.LastOperationTypeRestore, "Restoring the network"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Restore(ctx, network, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, network, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeRestore, "Error restoring network")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, network, gardencorev1beta1.LastOperationTypeRestore, "Successfully restored network"); err != nil {
		return reconcile.Result{}, err
	}

	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, network, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from network: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, network *extensionsv1alpha1.Network, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(network, FinalizerName) {
		r.logger.Info("Deleting network causes a no-op as there is no finalizer", "network", kutil.ObjectName(network))
		return reconcile.Result{}, nil
	}

	if err := r.statusUpdater.Processing(ctx, network, gardencorev1beta1.LastOperationTypeDelete, "Deleting the network"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the deletion of network", "network", kutil.ObjectName(network))
	if err := r.actuator.Delete(ctx, network, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, network, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeDelete, "Error deleting network")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, network, gardencorev1beta1.LastOperationTypeDelete, "Successfully deleted network"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Removing finalizer", "network", network.Name)
	if err := controllerutils.RemoveFinalizer(ctx, r.reader, r.client, network, FinalizerName); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizer from the network: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) migrate(ctx context.Context, network *extensionsv1alpha1.Network, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if err := r.statusUpdater.Processing(ctx, network, gardencorev1beta1.LastOperationTypeMigrate, "Migrating the network"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Migrate(ctx, network, cluster); err != nil {
		_ = r.statusUpdater.Error(ctx, network, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeMigrate, "Error migrating network")
		return reconcilerutils.ReconcileErr(err)
	}

	if err := r.statusUpdater.Success(ctx, network, gardencorev1beta1.LastOperationTypeMigrate, "Successfully migrated network"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Removing all finalizers", "network", kutil.ObjectName(network))
	if err := controllerutils.RemoveAllFinalizers(ctx, r.client, r.client, network); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizers from the network: %+v", err)
	}

	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, network, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from network: %+v", err)
	}

	return reconcile.Result{}, nil
}
