// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package conditions

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Reconciler reconciles Shoots registered as Seeds and maintains the Seeds conditions in the Shoot status.
type Reconciler struct {
	Client client.Client
	Config controllermanagerconfigv1alpha1.ShootConditionsControllerConfiguration
}

// Reconcile reconciles Shoots registered as Seeds and copies the Seed conditions to the Shoot object.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	shoot := &gardencorev1beta1.Shoot{}
	if err := r.Client.Get(ctx, request.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	// Get the seed this shoot is registered as
	seed, err := r.getShootSeed(ctx, shoot)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Build new shoot conditions
	// First remove all existing seed conditions and then add the current seed conditions if the shoot is still registered as seed.
	// The list of shoot conditions is well known (see contract https://github.com/gardener/gardener/blob/master/docs/extensions/shoot-health-status-conditions.md)
	// as opposed to seed conditions. Thus, subtract all shoot conditions to filter out the seed conditions.
	shootConditions := gardenerutils.GetShootConditionTypes(false)

	conditions := v1beta1helper.RetainConditions(shoot.Status.Conditions, shootConditions...)
	if seed != nil {
		conditions = v1beta1helper.MergeConditions(conditions, seed.Status.Conditions...)
	}

	// Update the shoot conditions if needed
	if v1beta1helper.ConditionsNeedUpdate(shoot.Status.Conditions, conditions) {
		log.V(1).Info("Updating shoot conditions")
		shoot.Status.Conditions = conditions
		// We are using Update here to ensure that we act upon an up-to-date version of the shoot.
		// An outdated cache together with a strategic merge patch can lead to incomplete patches if conditions change quickly.
		if err := r.Client.Status().Update(ctx, shoot); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) getShootSeed(ctx context.Context, shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Seed, error) {
	// Get the managed seed referencing this shoot
	ms, err := kubernetesutils.GetManagedSeedWithReader(ctx, r.Client, shoot.Namespace, shoot.Name)
	if err != nil || ms == nil {
		return nil, err
	}

	// Get the seed registered by the managed seed
	seed := &gardencorev1beta1.Seed{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: ms.Name}, seed); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return seed, nil
}
