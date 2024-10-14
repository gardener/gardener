// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/retry"
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
		g = flow.NewGraph("Extension deletion")

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

		cleanUpExtensionResources = g.Add(flow.Task{
			Name: "Wait for Extension Resources Cleanup",
			Fn: func(ctx context.Context) error {
				if err := retry.UntilTimeout(ctx, 5*time.Second, 120*time.Second, func(ctx context.Context) (done bool, err error) {
					if err := r.RuntimeClientSet.Client().Get(ctx, client.ObjectKeyFromObject(garden.garden), garden.garden); err != nil {
						return retry.SevereError(err)
					}
					if garden.garden.Annotations[operatorv1alpha1.AnnotationKeyExtensionResourcesCleanedUp] != "" {
						return retry.Ok()
					}
					log.Info("Waiting until extension resources are cleaned up")
					return retry.MinorError(fmt.Errorf("%s", "missing annotation marking extension resources as cleaned up"))
				}); err != nil {
					log.Error(err, "Error while waiting for extension resources cleaned up")
					return fmt.Errorf("waiting for extension resources cleanup failed: %s", err)
				}
				return nil
			},
			SkipIf: !garden.deleting,
		})

		_ = g.Add(flow.Task{
			Name: "Deleting Extension in runtime cluster",
			Fn: func(ctx context.Context) error {
				return r.runtime.Delete(ctx, log, extension)
			},
			Dependencies: flow.NewTaskIDs(cleanUpExtensionResources),
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
