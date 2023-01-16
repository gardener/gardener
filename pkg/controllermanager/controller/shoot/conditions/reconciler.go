// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Reconciler reconciles Shoots registered as Seeds and maintains the Seeds conditions in the Shoot status.
type Reconciler struct {
	Client client.Client
	Config config.ShootConditionsControllerConfiguration
}

// Reconcile reconciles Shoots registered as Seeds and copies the Seed conditions to the Shoot object.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := context.WithTimeout(ctx, controllerutils.DefaultReconciliationTimeout)
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
	// First remove all existing seed conditions and then add the current seed conditions
	// if the shoot is still registered as seed
	seedConditionTypes := []gardencorev1beta1.ConditionType{
		gardencorev1beta1.SeedBackupBucketsReady,
		gardencorev1beta1.SeedBootstrapped,
		gardencorev1beta1.SeedExtensionsReady,
		gardencorev1beta1.SeedGardenletReady,
		gardencorev1beta1.SeedSystemComponentsHealthy,
	}
	conditions := v1beta1helper.RemoveConditions(shoot.Status.Conditions, seedConditionTypes...)
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
	if err := r.Client.Get(ctx, kubernetesutils.Key(ms.Name), seed); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return seed, nil
}
