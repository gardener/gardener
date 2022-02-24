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

package quota

import (
	"context"
	"fmt"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (c *Controller) quotaAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.log.Error(err, "Couldn't get key for object", "object", obj)
		return
	}
	c.quotaQueue.Add(key)
}

func (c *Controller) quotaUpdate(oldObj, newObj interface{}) {
	c.quotaAdd(newObj)
}

func (c *Controller) quotaDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		c.log.Error(err, "Couldn't get key for object", "object", obj)
		return
	}
	c.quotaQueue.Add(key)
}

// NewQuotaReconciler creates a new instance of a reconciler which reconciles Quotas.
func NewQuotaReconciler(gardenClient client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &quotaReconciler{
		gardenClient: gardenClient,
		recorder:     recorder,
	}
}

type quotaReconciler struct {
	gardenClient client.Client
	recorder     record.EventRecorder
}

func (r *quotaReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	quota := &gardencorev1beta1.Quota{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, quota); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	// The deletionTimestamp labels a Quota as intended to get deleted. Before deletion,
	// it has to be ensured that no SecretBindings are depending on the Quota anymore.
	// When this happens the controller will remove the finalizers from the Quota so that it can be garbage collected.
	if quota.DeletionTimestamp != nil {
		if !sets.NewString(quota.Finalizers...).Has(gardencorev1beta1.GardenerName) {
			return reconcile.Result{}, nil
		}

		associatedSecretBindings, err := controllerutils.DetermineSecretBindingAssociations(ctx, r.gardenClient, quota)
		if err != nil {
			return reconcile.Result{}, err
		}

		if len(associatedSecretBindings) == 0 {
			log.Info("No SecretBindings are referencing the Quota, deletion accepted")

			// Remove finalizer from Quota
			if err := controllerutils.PatchRemoveFinalizers(ctx, r.gardenClient, quota, gardencorev1beta1.GardenerName); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed removing finalizer from quota: %w", err)
			}

			return reconcile.Result{}, nil
		}

		message := fmt.Sprintf("Cannot delete Quota, because the following SecretBindings are still referencing it: %+v", associatedSecretBindings)
		r.recorder.Event(quota, corev1.EventTypeNormal, v1beta1constants.EventResourceReferenced, message)
		return reconcile.Result{}, fmt.Errorf(message)
	}

	if !controllerutil.ContainsFinalizer(quota, gardencorev1beta1.GardenerName) {
		if err := controllerutils.StrategicMergePatchAddFinalizers(ctx, r.gardenClient, quota, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer to Quota: %w", err)
		}
	}

	return reconcile.Result{}, nil
}
