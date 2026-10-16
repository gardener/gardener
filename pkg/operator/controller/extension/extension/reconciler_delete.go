// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/gardener/operator"
)

func (r *Reconciler) delete(
	ctx context.Context,
	log logr.Logger,
	garden *gardenInfo,
	extension *operatorv1alpha1.Extension,
) (
	reconcile.Result,
	error,
) {
	apiServerDeploymentsFound, err := r.gardenAPIServersAvailable(ctx)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to check if the garden API servers are available: %w", err)
	}

	var (
		virtualClusterClientSet kubernetes.Interface

		reconcileResult reconcile.Result
		g               = flow.NewGraph("Extension deletion")

		_ = g.Add(flow.Task{
			Name: "Deleting ControllerRegistration and ControllerDeployment",
			Fn: func(ctx context.Context) error {
				return r.controllerRegistration.Delete(ctx, log, extension)
			},
		})

		_ = g.Add(flow.Task{
			Name: "Deleting Admission Controller",
			Fn: func(ctx context.Context) error {
				return r.admission.Delete(ctx, log, extension)
			},
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
			SkipIf: !apiServerDeploymentsFound || garden.garden == nil,
		})

		_ = g.Add(flow.Task{
			Name: "Handling Extension in runtime cluster",
			Fn: func(ctx context.Context) error {
				// Deploying the extension with a Garden kubeconfig during deletion is best-effort only, as the Garden cluster might already be in the process of being deleted.
				_, err := r.deployExtensionInRuntime(ctx, log, virtualClusterClientSet, extension)
				return err
			},
			Dependencies: flow.NewTaskIDs(createVirtualGardenClientSet),
		})
	)

	conditions := NewConditions(r.Clock, extension.Status)

	if err := g.Compile().Run(ctx, flow.Opts{
		Log: log,
	}); err != nil {
		conditions.installed = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.installed, gardencorev1beta1.ConditionFalse, ReasonDeleteFailed, err.Error())
		if updateErr := r.updateExtensionStatus(ctx, log, extension, conditions); updateErr != nil {
			return reconcile.Result{}, errors.Join(err, fmt.Errorf("failed to update extension status: %w", updateErr))
		}
		if !reflect.DeepEqual(reconcileResult, reconcile.Result{}) {
			return reconcileResult, nil
		}
		return reconcile.Result{}, err
	}

	if operator.IsExtensionInRuntimeRequired(extension) {
		conditions.installed = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.installed, gardencorev1beta1.ConditionTrue, ReasonInstalledInRuntime, "Extension is still required in runtime cluster")
		return reconcile.Result{}, r.updateExtensionStatus(ctx, log, extension, conditions)
	}

	conditions.installed = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.installed, gardencorev1beta1.ConditionFalse, ReasonDeleteSuccessful, "Extension has been deleted successfully")
	if err := r.updateExtensionStatus(ctx, log, extension, conditions); err != nil {
		return reconcile.Result{}, err
	}

	if extension.DeletionTimestamp != nil {
		return reconcile.Result{}, r.removeFinalizer(ctx, log, extension)
	}

	return reconcile.Result{}, nil
}
