// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package secretbinding

import (
	"context"
	"fmt"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

const providerTypeReconcilerName = "provider-type"

func (c *Controller) shootAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.log.Error(err, "Couldn't get key for object", "object", obj)
		return
	}
	c.shootQueue.Add(key)
}

// NewSecretBindingProviderReconciler creates a new instance of a reconciler which populates
// the SecretBinding provider type based on the Shoot provider type.
func NewSecretBindingProviderReconciler(gardenClient client.Client) reconcile.Reconciler {
	return &secretBindingProviderReconciler{
		gardenClient: gardenClient,
	}
}

type secretBindingProviderReconciler struct {
	gardenClient client.Client
}

func (r *secretBindingProviderReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	shoot := &gardencorev1beta1.Shoot{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	secretBinding := &gardencorev1beta1.SecretBinding{}
	if err := r.gardenClient.Get(ctx, kutil.Key(shoot.Namespace, shoot.Spec.SecretBindingName), secretBinding); err != nil {
		return reconcile.Result{}, err
	}

	shootProviderType := shoot.Spec.Provider.Type
	log = log.WithValues("secretBinding", client.ObjectKeyFromObject(secretBinding), "shootProviderType", shootProviderType)

	if secretBinding.Provider != nil && gardencorev1beta1helper.SecretBindingHasType(secretBinding, shootProviderType) {
		log.V(1).Info("SecretBinding already has provider type, nothing to do")
		return reconcile.Result{}, nil
	}

	log.Info("SecretBinding does not have provider type, adding it")

	patch := client.MergeFromWithOptions(secretBinding.DeepCopy(), client.MergeFromWithOptimisticLock{})
	gardencorev1beta1helper.AddTypeToSecretBinding(secretBinding, shootProviderType)
	return reconcile.Result{}, r.gardenClient.Patch(ctx, secretBinding, patch)
}
