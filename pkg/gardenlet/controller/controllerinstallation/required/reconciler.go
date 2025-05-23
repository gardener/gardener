// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package required

import (
	"context"
	"fmt"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
)

// Reconciler reconciles ControllerInstallations. It checks whether they are still required by using the
// <KindToRequiredTypes> map.
type Reconciler struct {
	GardenClient client.Client
	SeedClient   client.Client
	Config       gardenletconfigv1alpha1.ControllerInstallationRequiredControllerConfiguration
	Clock        clock.Clock
	SeedName     string

	Lock                *sync.RWMutex
	KindToRequiredTypes map[string]sets.Set[string]
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	controllerInstallation := &gardencorev1beta1.ControllerInstallation{}
	if err := r.GardenClient.Get(ctx, request.NamespacedName, controllerInstallation); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	controllerRegistration := &gardencorev1beta1.ControllerRegistration{}
	if err := r.GardenClient.Get(ctx, client.ObjectKey{Name: controllerInstallation.Spec.RegistrationRef.Name}, controllerRegistration); err != nil {
		return reconcile.Result{}, err
	}

	var (
		allKindsCalculated = true
		required           *bool
		requiredKindTypes  = sets.New[string]()
		message            string
	)

	r.Lock.RLock()
	for _, resource := range controllerRegistration.Spec.Resources {
		requiredTypes, ok := r.KindToRequiredTypes[resource.Kind]
		if !ok {
			allKindsCalculated = false
			continue
		}

		if requiredTypes.Has(resource.Type) {
			required = ptr.To(true)
			requiredKindTypes.Insert(fmt.Sprintf("%s/%s", resource.Kind, resource.Type))
		}
	}
	r.Lock.RUnlock()

	if required == nil {
		if !allKindsCalculated {
			// if required wasn't set yet then but not all kinds were calculated then the it's not possible to
			// decide yet whether it's required or not
			return reconcile.Result{}, nil
		}

		// if required wasn't set yet then but all kinds were calculated then the installation is no longer required
		required = ptr.To(false)
		message = "no extension objects exist in the seed having the kind/type combinations the controller is responsible for"
	} else if *required {
		message = fmt.Sprintf("extension objects still exist in the seed: %+v", requiredKindTypes.UnsortedList())
	}

	if err := updateControllerInstallationRequiredCondition(ctx, r.GardenClient, r.Clock, controllerInstallation, *required, message); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func updateControllerInstallationRequiredCondition(ctx context.Context, c client.StatusClient, clock clock.Clock, controllerInstallation *gardencorev1beta1.ControllerInstallation, required bool, message string) error {
	var (
		conditionRequired = v1beta1helper.GetOrInitConditionWithClock(clock, controllerInstallation.Status.Conditions, gardencorev1beta1.ControllerInstallationRequired)

		status = gardencorev1beta1.ConditionTrue
		reason = "ExtensionObjectsExist"
	)

	if !required {
		status = gardencorev1beta1.ConditionFalse
		reason = "NoExtensionObjects"
	}

	patch := client.StrategicMergeFrom(controllerInstallation.DeepCopy())
	controllerInstallation.Status.Conditions = v1beta1helper.MergeConditions(
		controllerInstallation.Status.Conditions,
		v1beta1helper.UpdatedConditionWithClock(clock, conditionRequired, status, reason, message),
	)

	return c.Status().Patch(ctx, controllerInstallation, patch)
}
