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

package bastion

import (
	"context"
	"fmt"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Reconciler reconciles Bastions.
type Reconciler struct {
	Client client.Client
	Config config.BastionControllerConfiguration
	Clock  clock.Clock
}

// Reconcile reacts to updates on Bastion resources and cleans up expired Bastions.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := context.WithTimeout(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	bastion := &operationsv1alpha1.Bastion{}
	if err := r.Client.Get(ctx, request.NamespacedName, bastion); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	// do not reconcile anymore once the object is marked for deletion
	if bastion.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}

	shootKey := kubernetesutils.Key(bastion.Namespace, bastion.Spec.ShootRef.Name)
	log = log.WithValues("shoot", shootKey)

	// fetch associated Shoot
	shoot := gardencorev1beta1.Shoot{}
	if err := r.Client.Get(ctx, shootKey, &shoot); err != nil {
		// This should never happen, as the shoot deletion is stopped unless all Bastions
		// are removed. This is required because once a Shoot is gone, the Cluster resource
		// is gone as well and without that, cleanly destroying a Bastion is not possible.
		if apierrors.IsNotFound(err) {
			log.Info("Deleting bastion because target shoot is gone")
			return reconcile.Result{}, client.IgnoreNotFound(r.Client.Delete(ctx, bastion))
		}
		return reconcile.Result{}, fmt.Errorf("could not get shoot %v: %w", shootKey, err)
	}

	// delete the bastion if the shoot is marked for deletion
	if shoot.DeletionTimestamp != nil {
		log.Info("Deleting bastion because target shoot is in deletion")
		return reconcile.Result{}, client.IgnoreNotFound(r.Client.Delete(ctx, bastion))
	}

	// the Shoot for this bastion has been migrated to another Seed, we have to garbage-collect
	// the old bastion (bastions are not migrated, users are required to create new bastions);
	// equality is the correct check here, as the admission plugin already prevents Bastions
	// from existing without a spec.SeedName being set. So it cannot happen that we accidentally
	// delete a Bastion without seed (i.e. an unreconciled, new Bastion);
	// under normal operations, shoots cannot be migrated to another seed while there are still
	// bastions for it, so this check here is just a safety measure.
	if !apiequality.Semantic.DeepEqual(shoot.Spec.SeedName, bastion.Spec.SeedName) {
		log.Info("Deleting bastion because the referenced Shoot has been migrated to another Seed",
			"oldSeedName", bastion.Spec.SeedName, "newSeedName", shoot.Spec.SeedName)
		return reconcile.Result{}, client.IgnoreNotFound(r.Client.Delete(ctx, bastion))
	}

	// delete the bastion once it has expired
	if bastion.Status.ExpirationTimestamp != nil && r.Clock.Now().After(bastion.Status.ExpirationTimestamp.Time) {
		log.Info("Deleting expired bastion", "expirationTimestamp", bastion.Status.ExpirationTimestamp.Time)
		return reconcile.Result{}, client.IgnoreNotFound(r.Client.Delete(ctx, bastion))
	}

	// delete the bastion once it has reached its maximum lifetime
	if r.Clock.Since(bastion.CreationTimestamp.Time) > r.Config.MaxLifetime.Duration {
		log.Info("Deleting bastion because it reached its maximum lifetime", "creationTimestamp", bastion.CreationTimestamp.Time, "maxLifetime", r.Config.MaxLifetime.Duration)
		return reconcile.Result{}, client.IgnoreNotFound(r.Client.Delete(ctx, bastion))
	}

	// requeue when the Bastion expires or reaches its lifetime, whichever is sooner
	requeueAfter := bastion.CreationTimestamp.Add(r.Config.MaxLifetime.Duration).Sub(r.Clock.Now())
	if bastion.Status.ExpirationTimestamp != nil {
		expiresIn := bastion.Status.ExpirationTimestamp.Sub(r.Clock.Now())
		if expiresIn < requeueAfter {
			requeueAfter = expiresIn
		}
	}

	if requeueAfter < 0 {
		return reconcile.Result{}, fmt.Errorf("the bastion should already have been deleted")
	}

	log.V(1).Info("Requeuing Bastion", "requeueAfter", requeueAfter)
	return reconcile.Result{RequeueAfter: requeueAfter}, nil
}
