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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/logger"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (c *Controller) shootAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.shootQueue.Add(key)
}

// NewSecretBindingProviderPopulatorReconciler creates a new instance of a reconciler which populates
// the SecretBinding provider type based on the Shoot provider type.
func NewSecretBindingProviderPopulatorReconciler(l logrus.FieldLogger, gardenClient client.Client) reconcile.Reconciler {
	return &secretBindingProviderPopulatorReconciler{
		logger:       l,
		gardenClient: gardenClient,
	}
}

type secretBindingProviderPopulatorReconciler struct {
	logger       logrus.FieldLogger
	gardenClient client.Client
}

func (r *secretBindingProviderPopulatorReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
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

	secretBindingLogger := logger.NewFieldLogger(r.logger, "secretbinding", fmt.Sprintf("%s/%s", secretBinding.Namespace, secretBinding.Name))

	shootProviderType := shoot.Spec.Provider.Type
	if secretBinding.Provider != nil && gardencorev1beta1helper.SecretBindingHasType(secretBinding, shootProviderType) {
		secretBindingLogger.Debugf("SecretBinding already has provider type '%s'. Nothing to do.", shootProviderType)
		return reconcile.Result{}, nil
	}

	secretBindingLogger.Debugf("SecretBinding does not have provider type '%s'. Will add it.", shootProviderType)

	patch := client.MergeFromWithOptions(secretBinding.DeepCopy(), client.MergeFromWithOptimisticLock{})
	gardencorev1beta1helper.AddTypeToSecretBinding(secretBinding, shootProviderType)

	if err := r.gardenClient.Patch(ctx, secretBinding, patch); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
