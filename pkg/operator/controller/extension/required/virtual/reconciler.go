// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package virtual

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	operatorconfigv1alpha1 "github.com/gardener/gardener/pkg/operator/apis/config/v1alpha1"
)

// Reconciler reconciles Extensions to determine their required state.
type Reconciler struct {
	Config        operatorconfigv1alpha1.ExtensionRequiredVirtualControllerConfiguration
	RuntimeClient client.Client
	VirtualClient client.Client

	clock clock.Clock
}

// Reconcile processes the given extension object in the request.
// It lists required ControllerInstallations to ascertain if the extension is required in seed clusters.
// At the end, the RequiredVirtual condition is updated for the extension.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	extension := &operatorv1alpha1.Extension{}
	if err := r.RuntimeClient.Get(ctx, request.NamespacedName, extension); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	controllerInstallation := &gardencorev1beta1.ControllerInstallationList{}
	if err := r.VirtualClient.List(ctx, controllerInstallation, client.MatchingFields{gardencore.RegistrationRefName: extension.Name}); err != nil {
		return reconcile.Result{}, fmt.Errorf("error listing controllerinstallations: %w", err)
	}

	var required bool
	for _, ctrlInstallation := range controllerInstallation.Items {
		requiredCondition := v1beta1helper.GetCondition(ctrlInstallation.Status.Conditions, gardencorev1beta1.ControllerInstallationRequired)
		if requiredCondition != nil && requiredCondition.Status == gardencorev1beta1.ConditionTrue {
			required = true
			break
		}
	}

	if err := r.updateCondition(ctx, log, extension, required); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not update extension status: %w", err)
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) updateCondition(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension, extensionRequired bool) error {
	requiredCondition := v1beta1helper.GetOrInitConditionWithClock(r.clock, extension.Status.Conditions, operatorv1alpha1.ExtensionRequiredVirtual)

	if extensionRequired {
		requiredCondition = v1beta1helper.UpdatedConditionWithClock(r.clock, requiredCondition, gardencorev1beta1.ConditionTrue, "RequiredControllerInstallation", "Extension has required ControllerInstallations for seed clusters")
		log.Info("Extension required for seed cluster")
	} else {
		requiredCondition = v1beta1helper.UpdatedConditionWithClock(r.clock, requiredCondition, gardencorev1beta1.ConditionFalse, "NoRequiredControllerInstallation", "Extension does not have required ControllerInstallations for seed clusters")
		log.Info("Extension not required for seed cluster")
	}

	patch := client.MergeFromWithOptions(extension.DeepCopy(), client.MergeFromWithOptimisticLock{})
	newConditions := v1beta1helper.MergeConditions(extension.Status.Conditions, requiredCondition)
	if !v1beta1helper.ConditionsNeedUpdate(extension.Status.Conditions, newConditions) {
		return nil
	}

	extension.Status.Conditions = newConditions
	return r.RuntimeClient.Status().Patch(ctx, extension, patch)
}
