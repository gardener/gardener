// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package quota

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Reconciler reconciles Shoots and auto-deletes them if they are bound to a Quota with a configured cluster lifetime.
type Reconciler struct {
	Client client.Client
	Config config.ShootQuotaControllerConfiguration
	Clock  clock.Clock
}

// Reconcile reconciles Shoots and auto-deletes them if they are bound to a Quota with a configured cluster lifetime.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, r.Config.SyncPeriod.Duration)
	defer cancel()

	shoot := &gardencorev1beta1.Shoot{}
	if err := r.Client.Get(ctx, request.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	var (
		secretBinding   = &gardencorev1beta1.SecretBinding{}
		clusterLifeTime *int32
	)

	if shoot.Spec.SecretBindingName != nil {
		if err := r.Client.Get(ctx, kubernetesutils.Key(shoot.Namespace, *shoot.Spec.SecretBindingName), secretBinding); err != nil {
			return reconcile.Result{}, err
		}
	}

	for _, quotaRef := range secretBinding.Quotas {
		quota := &gardencorev1beta1.Quota{}
		if err := r.Client.Get(ctx, kubernetesutils.Key(quotaRef.Namespace, quotaRef.Name), quota); err != nil {
			return reconcile.Result{}, err
		}

		if quota.Spec.ClusterLifetimeDays == nil {
			continue
		}
		if clusterLifeTime == nil || *quota.Spec.ClusterLifetimeDays < *clusterLifeTime {
			clusterLifeTime = quota.Spec.ClusterLifetimeDays
		}
	}

	// If the Shoot has no Quotas referenced (anymore) or if the referenced Quotas does not have a clusterLifetime,
	// then we will not check for cluster lifetime expiration, even if the Shoot has a clusterLifetime timestamp already
	// annotated.
	if clusterLifeTime == nil {
		if metav1.HasAnnotation(shoot.ObjectMeta, v1beta1constants.ShootExpirationTimestamp) {
			log.Info("Removing expiration timestamp annotation")

			patch := client.MergeFrom(shoot.DeepCopy())
			delete(shoot.Annotations, v1beta1constants.ShootExpirationTimestamp)
			if err := r.Client.Patch(ctx, shoot, patch); err != nil {
				return reconcile.Result{}, err
			}
		}
		return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
	}

	expirationTime, exist := shoot.Annotations[v1beta1constants.ShootExpirationTimestamp]
	if !exist {
		expirationTime = shoot.CreationTimestamp.Add(time.Duration(*clusterLifeTime*24) * time.Hour).Format(time.RFC3339)
		log.Info("Setting expiration timestamp annotation", "expirationTime", expirationTime)

		patch := client.MergeFrom(shoot.DeepCopy())
		metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.ShootExpirationTimestamp, expirationTime)
		if err := r.Client.Patch(ctx, shoot, patch); err != nil {
			return reconcile.Result{}, err
		}
	}

	expirationTimeParsed, err := time.Parse(time.RFC3339, expirationTime)
	if err != nil {
		return reconcile.Result{}, err
	}

	if r.Clock.Now().UTC().After(expirationTimeParsed.UTC()) {
		log.Info("Shoot cluster lifetime expired, deleting Shoot", "expirationTime", expirationTime)

		// We have to annotate the Shoot to confirm the deletion.
		if err := gardenerutils.ConfirmDeletion(ctx, r.Client, shoot, false); err != nil {
			if apierrors.IsNotFound(err) {
				return reconcile.Result{}, nil
			}
			return reconcile.Result{}, err
		}

		// Now we are allowed to delete the Shoot (to set the deletionTimestamp).
		return reconcile.Result{}, client.IgnoreNotFound(r.Client.Delete(ctx, shoot))
	}

	return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
}
