// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/flow"
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
	deleteCtx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	var (
		virtualClusterClientSet kubernetes.Interface
		g                       = flow.NewGraph("Extension deletion")

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
			SkipIf: garden.garden == nil,
		})

		_ = g.Add(flow.Task{
			Name: "Deleting ControllerRegistration and ControllerDeployment",
			Fn: func(ctx context.Context) error {
				return r.controllerRegistration.Delete(ctx, log, extension)
			},
		})

		_ = g.Add(flow.Task{
			Name: "Deleting Admission Controller",
			Fn: func(ctx context.Context) error {
				return r.admission.Delete(ctx, log, virtualClusterClientSet, extension)
			},
			Dependencies: flow.NewTaskIDs(createVirtualGardenClientSet),
		})

		_ = g.Add(flow.Task{
			Name: "Deleting Extension in runtime cluster",
			Fn: func(ctx context.Context) error {
				return r.runtime.Delete(ctx, log, extension)
			},
		})
	)

	conditions := NewConditions(r.Clock, extension.Status)

	if err := g.Compile().Run(deleteCtx, flow.Opts{
		Log: log,
	}); err != nil {
		conditions.installed = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.installed, gardencorev1beta1.ConditionFalse, ConditionDeleteFailed, err.Error())
		return reconcile.Result{}, errors.Join(err, r.updateExtensionStatus(ctx, log, extension, conditions))
	}

	conditions.installed = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditions.installed, gardencorev1beta1.ConditionFalse, ConditionDeleteSuccessful, "Extension has been deleted successfully")
	if err := r.updateExtensionStatus(ctx, log, extension, conditions); err != nil {
		return reconcile.Result{}, err
	}

	if extension.DeletionTimestamp != nil {
		return reconcile.Result{}, r.removeFinalizer(ctx, log, extension)
	}

	return reconcile.Result{}, nil
}
