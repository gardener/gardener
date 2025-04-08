// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/gardener"
)

// RequeueGardenResourceNotReady is the time after which an extension will be requeued, if the Garden resource was not ready during its reconciliation. Exposed for testing.
var RequeueGardenResourceNotReady = 10 * time.Second

func (r *Reconciler) reconcile(
	ctx context.Context,
	log logr.Logger,
	garden *gardenInfo,
	extension *operatorv1alpha1.Extension,
) (
	reconcile.Result,
	error,
) {
	conditions := NewConditions(r.Clock, extension.Status)

	if garden.garden == nil {
		if conditions.installed.Status == gardencorev1beta1.ConditionFalse && conditions.installed.Reason == ReasonDeleteSuccessful {
			// Retain condition if the extension was previously uninstalled due to a garden deletion
			return reconcile.Result{}, nil
		}

		conditions.installed = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.installed, gardencorev1beta1.ConditionFalse, ReasonNoGardenFound, "No garden found")
		return reconcile.Result{}, r.updateExtensionStatus(ctx, log, extension, conditions)
	}

	reconcileCtx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	if !controllerutil.ContainsFinalizer(extension, operatorv1alpha1.FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(reconcileCtx, r.RuntimeClientSet.Client(), extension, operatorv1alpha1.FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	var (
		virtualClusterClientSet kubernetes.Interface
		reconcileResult         reconcile.Result
		g                       = flow.NewGraph("Extension reconciliation")

		deployExtensionInRuntime = g.Add(flow.Task{
			Name: "Deploying extension in runtime cluster",
			Fn: func(ctx context.Context) error {
				return r.deployExtensionInRuntime(ctx, log, extension)
			},
		})

		checkGarden = g.Add(flow.Task{
			Name: "Checking if garden is reconciled",
			Fn: func(_ context.Context) error {
				if !garden.reconciled {
					log.Info("Garden is not yet in 'Reconcile Succeeded' state, re-queueing", "requeueAfter", RequeueGardenResourceNotReady)
					reconcileResult = reconcile.Result{RequeueAfter: RequeueGardenResourceNotReady}
					return fmt.Errorf("garden is not yet successfully reconciled")
				}
				return nil
			},
			Dependencies: flow.NewTaskIDs(deployExtensionInRuntime),
		})

		createVirtualGardenClientSet = g.Add(flow.Task{
			Name: "Creating virtual garden-client",
			Fn: func(ctx context.Context) error {
				clientSet, err := r.GardenClientMap.GetClient(ctx, keys.ForGarden(garden.garden))
				if err != nil {
					return fmt.Errorf("error retrieving virtual cluster client set: %w", err)
				}

				virtualClusterClientSet = clientSet
				return nil
			},
			Dependencies: flow.NewTaskIDs(checkGarden),
		})

		_ = g.Add(flow.Task{
			Name: "Deploying Admission Controller",
			Fn: func(ctx context.Context) error {
				if garden.genericTokenKubeconfigSecretName == nil {
					return fmt.Errorf("generic kubeconfig secret name is not set for garden")
				}
				return r.admission.Reconcile(ctx, log, virtualClusterClientSet, *garden.genericTokenKubeconfigSecretName, extension)
			},
			Dependencies: flow.NewTaskIDs(createVirtualGardenClientSet),
		})

		_ = g.Add(flow.Task{
			Name: "Deploying ControllerRegistration and ControllerDeployment",
			Fn: func(ctx context.Context) error {
				return r.controllerRegistration.Reconcile(ctx, log, extension)
			},
			Dependencies: flow.NewTaskIDs(checkGarden),
		})
	)

	if err := g.Compile().Run(reconcileCtx, flow.Opts{
		Log: log,
	}); err != nil {
		conditions.installed = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.installed, gardencorev1beta1.ConditionFalse, ReasonReconcileFailed, err.Error())
		if err := r.updateExtensionStatus(ctx, log, extension, conditions); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to update extension status: %w", err)
		}
		if !reflect.DeepEqual(reconcileResult, reconcile.Result{}) {
			return reconcileResult, nil
		}
		return reconcile.Result{}, errors.Join(err, r.updateExtensionStatus(ctx, log, extension, conditions))
	}

	conditions.installed = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.installed, gardencorev1beta1.ConditionTrue, ReasonReconcileSuccess, "Extension has been reconciled successfully")
	return reconcileResult, r.updateExtensionStatus(ctx, log, extension, conditions)
}

func (r *Reconciler) deployExtensionInRuntime(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension) error {
	if !gardener.IsExtensionInRuntimeRequired(extension) {
		log.V(1).Info("Deployment in runtime cluster not required")
		return r.runtime.Delete(ctx, log, extension)
	}
	log.V(1).Info("Deployment in runtime cluster required")
	return r.runtime.Reconcile(ctx, log, extension)
}
