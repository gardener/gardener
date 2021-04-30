// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot

import (
	"context"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (c *Controller) shootQuotaAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.shootQuotaQueue.Add(key)
}

func (c *Controller) shootQuotaDelete(obj interface{}) {
	shoot, ok := obj.(*gardencorev1beta1.Shoot)
	if shoot == nil || !ok {
		return
	}
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.shootQuotaQueue.Done(key)
}

// NewShootQuotaReconciler creates a new instance of a reconciler which checks handles Shoots using SecretBindings that
// references Quotas.
func NewShootQuotaReconciler(l logrus.FieldLogger, gardenClient client.Client, cfg config.ShootQuotaControllerConfiguration) reconcile.Reconciler {
	return &shootQuotaReconciler{
		logger:       l,
		cfg:          cfg,
		gardenClient: gardenClient,
	}
}

type shootQuotaReconciler struct {
	logger       logrus.FieldLogger
	cfg          config.ShootQuotaControllerConfiguration
	gardenClient client.Client
}

func (r *shootQuotaReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	shoot := &gardencorev1beta1.Shoot{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Infof("Object %q is gone, stop reconciling: %v", request.Name, err)
			return reconcile.Result{}, nil
		}
		r.logger.Infof("Unable to retrieve object %q from store: %v", request.Name, err)
		return reconcile.Result{}, err
	}

	secretBinding := &gardencorev1beta1.SecretBinding{}
	if err := r.gardenClient.Get(ctx, kutil.Key(shoot.Namespace, shoot.Spec.SecretBindingName), secretBinding); err != nil {
		return reconcile.Result{}, err
	}

	var clusterLifeTime *int32

	for _, quotaRef := range secretBinding.Quotas {
		quota := &gardencorev1beta1.Quota{}
		if err := r.gardenClient.Get(ctx, kutil.Key(quotaRef.Namespace, quotaRef.Name), quota); err != nil {
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
	// then we will not check for cluster lifetime expiration, even if the Shoot has a clusterLifetime timestamp already annotated.
	if clusterLifeTime == nil {
		return reconcile.Result{RequeueAfter: r.cfg.SyncPeriod.Duration}, nil
	}

	expirationTime, exits := shoot.Annotations[v1beta1constants.ShootExpirationTimestamp]
	if !exits {
		expirationTime = shoot.CreationTimestamp.Add(time.Duration(*clusterLifeTime*24) * time.Hour).Format(time.RFC3339)
		metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.ShootExpirationTimestamp, expirationTime)

		if err := r.gardenClient.Update(ctx, shoot); err != nil {
			return reconcile.Result{}, err
		}
	}

	expirationTimeParsed, err := time.Parse(time.RFC3339, expirationTime)
	if err != nil {
		return reconcile.Result{}, err
	}

	if time.Now().UTC().After(expirationTimeParsed.UTC()) {
		r.logger.Info("[SHOOT QUOTA] Shoot cluster lifetime expired. Shoot will be deleted.")

		// We have to annotate the Shoot to confirm the deletion.
		if err := gutil.ConfirmDeletion(ctx, r.gardenClient, shoot); err != nil {
			if apierrors.IsNotFound(err) {
				r.logger.Info("Shoot already gone")
				return reconcile.Result{}, nil
			}
			return reconcile.Result{}, err
		}

		// Now we are allowed to delete the Shoot (to set the deletionTimestamp).
		return reconcile.Result{}, client.IgnoreNotFound(r.gardenClient.Delete(ctx, shoot))
	}

	return reconcile.Result{RequeueAfter: r.cfg.SyncPeriod.Duration}, nil
}
