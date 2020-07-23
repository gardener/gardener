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

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	contextutil "github.com/gardener/gardener/pkg/utils/context"

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
	// EventNetworkReconciliation an event reason to describe network reconciliation.
	EventNetworkReconciliation string = "NetworkReconciliation"
	// EventNetworkDeletion an event reason to describe network deletion.
	EventNetworkDeletion string = "NetworkDeletion"
	// EventNetworkRestoration an event reason to describe network restoration.
	EventNetworkRestoration string = "NetworkRestoration"
	// EventNetworkMigartion an event reason to describe network migration.
	EventNetworkMigartion string = "NetworkMigration"
)

type reconciler struct {
	logger   logr.Logger
	actuator Actuator

	ctx      context.Context
	client   client.Client
	recorder record.EventRecorder
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// Network resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(mgr manager.Manager, actuator Actuator) reconcile.Reconciler {
	return extensionscontroller.OperationAnnotationWrapper(
		&extensionsv1alpha1.Network{},
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
	r.ctx = contextutil.FromStopChannel(stopCh)
	return nil
}

func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	network := &extensionsv1alpha1.Network{}
	if err := r.client.Get(r.ctx, request.NamespacedName, network); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	cluster, err := extensionscontroller.GetCluster(r.ctx, r.client, network.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}

	if extensionscontroller.IsFailed(cluster) {
		r.logger.Info("Stop reconciling Network of failed Shoot.", "namespace", request.Namespace, "name", network.Name)
		return reconcile.Result{}, nil
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(network.ObjectMeta, network.Status.LastOperation)

	switch {
	case extensionscontroller.IsMigrated(network):
		return reconcile.Result{}, nil
	case operationType == gardencorev1beta1.LastOperationTypeMigrate:
		return r.migrate(r.ctx, network, cluster)
	case network.DeletionTimestamp != nil:
		return r.delete(r.ctx, network, cluster)
	case network.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRestore:
		return r.restore(r.ctx, network, cluster)
	default:
		return r.reconcile(r.ctx, network, cluster, operationType)
	}
}

func (r *reconciler) reconcile(ctx context.Context, network *extensionsv1alpha1.Network, cluster *extensionscontroller.Cluster, operationType gardencorev1beta1.LastOperationType) (reconcile.Result, error) {
	if err := extensionscontroller.EnsureFinalizer(ctx, r.client, FinalizerName, network); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.updateStatusProcessing(ctx, network, operationType, EventNetworkReconciliation, "Reconciling the network"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Reconcile(ctx, network, cluster); err != nil {
		utilruntime.HandleError(r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), network, operationType, EventNetworkReconciliation, "Error reconciling network"))
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.updateStatusSuccess(ctx, network, operationType, EventNetworkReconciliation, "Successfully reconciled network"); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) restore(ctx context.Context, network *extensionsv1alpha1.Network, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if err := extensionscontroller.EnsureFinalizer(ctx, r.client, FinalizerName, network); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.updateStatusProcessing(ctx, network, gardencorev1beta1.LastOperationTypeRestore, EventNetworkRestoration, "Restoring the network"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Restore(ctx, network, cluster); err != nil {
		utilruntime.HandleError(r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), network, gardencorev1beta1.LastOperationTypeRestore, EventNetworkRestoration, "Error restoring network"))
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.updateStatusSuccess(ctx, network, gardencorev1beta1.LastOperationTypeRestore, EventNetworkRestoration, "Successfully restored network"); err != nil {
		return reconcile.Result{}, err
	}

	// remove operation annotation 'restore'
	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, network, v1beta1constants.GardenerOperation); err != nil {
		msg := "Error removing annotation from Network"
		r.recorder.Eventf(network, corev1.EventTypeWarning, EventNetworkRestoration, "%s: %+v", msg, err)
		return reconcile.Result{}, fmt.Errorf("%s: %+v", msg, err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, network *extensionsv1alpha1.Network, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	hasFinalizer, err := extensionscontroller.HasFinalizer(network, FinalizerName)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not instantiate finalizer deletion: %+v", err)
	}
	if !hasFinalizer {
		r.logger.Info("Deleting network causes a no-op as there is no finalizer.", "network", network.Name)
		return reconcile.Result{}, nil
	}

	if err := r.updateStatusProcessing(ctx, network, gardencorev1beta1.LastOperationTypeDelete, EventNetworkDeletion, "Deleting the network"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Starting the deletion of network", "network", network.Name)
	r.recorder.Event(network, corev1.EventTypeNormal, EventNetworkDeletion, "Deleting the network")
	if err := r.actuator.Delete(r.ctx, network, cluster); err != nil {
		utilruntime.HandleError(r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), network, gardencorev1beta1.LastOperationTypeDelete, EventNetworkDeletion, "Error deleting network"))
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.updateStatusSuccess(ctx, network, gardencorev1beta1.LastOperationTypeDelete, EventNetworkDeletion, "Successfully deleted network"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Removing finalizer.", "network", network.Name)
	if err := extensionscontroller.DeleteFinalizer(ctx, r.client, FinalizerName, network); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizer from the Network resource: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) migrate(ctx context.Context, network *extensionsv1alpha1.Network, cluster *extensionscontroller.Cluster) (reconcile.Result, error) {
	if err := r.updateStatusProcessing(ctx, network, gardencorev1beta1.LastOperationTypeMigrate, EventNetworkMigartion, "Migrating the network"); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.actuator.Migrate(r.ctx, network, cluster); err != nil {
		utilruntime.HandleError(r.updateStatusError(ctx, extensionscontroller.ReconcileErrCauseOrErr(err), network, gardencorev1beta1.LastOperationTypeMigrate, EventNetworkMigartion, "Error migrating network"))
		return extensionscontroller.ReconcileErr(err)
	}

	if err := r.updateStatusSuccess(ctx, network, gardencorev1beta1.LastOperationTypeMigrate, EventNetworkMigartion, "Successfully migrated network"); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Removing all finalizers.", "network", network.Name)
	if err := extensionscontroller.DeleteAllFinalizers(ctx, r.client, network); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizers from the Network resource: %+v", err)
	}

	// remove operation annotation 'migrate'
	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, network, v1beta1constants.GardenerOperation); err != nil {
		msg := "Error removing annotation from Network"
		r.recorder.Eventf(network, corev1.EventTypeWarning, EventNetworkRestoration, "%s: %+v", msg, err)
		return reconcile.Result{}, fmt.Errorf("%s: %+v", msg, err)
	}
	return reconcile.Result{}, nil
}

func (r *reconciler) updateStatusProcessing(ctx context.Context, network *extensionsv1alpha1.Network, lastOperationType gardencorev1beta1.LastOperationType, eventReason, description string) error {
	r.logger.Info(description, "network", network.Name)
	r.recorder.Event(network, corev1.EventTypeNormal, eventReason, description)
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, network, func() error {
		network.Status.LastOperation = extensionscontroller.LastOperation(lastOperationType, gardencorev1beta1.LastOperationStateProcessing, 1, description)
		return nil
	})
}

func (r *reconciler) updateStatusError(ctx context.Context, err error, network *extensionsv1alpha1.Network, lastOperationType gardencorev1beta1.LastOperationType, eventReason, description string) error {
	r.recorder.Eventf(network, corev1.EventTypeWarning, eventReason, "%s: %+v", description, err)
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, network, func() error {
		network.Status.ObservedGeneration = network.Generation
		network.Status.LastOperation, network.Status.LastError = extensionscontroller.ReconcileError(lastOperationType, gardencorev1beta1helper.FormatLastErrDescription(fmt.Errorf("%s: %v", description, err)), 50, gardencorev1beta1helper.ExtractErrorCodes(err)...)
		return nil
	})
}

func (r *reconciler) updateStatusSuccess(ctx context.Context, network *extensionsv1alpha1.Network, lastOperationType gardencorev1beta1.LastOperationType, eventReason, description string) error {
	r.logger.Info(description, "network", network.Name)
	r.recorder.Event(network, corev1.EventTypeNormal, eventReason, description)
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, network, func() error {
		network.Status.ObservedGeneration = network.Generation
		network.Status.LastOperation, network.Status.LastError = extensionscontroller.ReconcileSucceeded(lastOperationType, description)
		return nil
	})
}
