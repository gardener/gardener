// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/gardener/operator"
)

func (r *Reconciler) delete(
	ctx context.Context,
	log logr.Logger,
	extension *operatorv1alpha1.Extension,
) (
	reconcile.Result,
	error,
) {
	var (
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

		_ = g.Add(flow.Task{
			Name: "Handling Extension in runtime cluster",
			Fn: func(ctx context.Context) error {
				var err error
				reconcileResult, err = r.deployExtensionInRuntime(ctx, log, extension)
				return err
			},
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
