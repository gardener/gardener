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

package shoot

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/logger"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (c *Controller) shootConditionsAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.shootConditionsQueue.Add(key)
}

func (c *Controller) filterSeedForShootConditions(obj, oldObj, _ client.Object, deleted bool) bool {
	seed, ok := obj.(*gardencorev1beta1.Seed)
	if !ok {
		return false
	}

	if oldObj != nil {
		oldSeed, ok := oldObj.(*gardencorev1beta1.Seed)
		if !ok {
			return false
		}

		if !apiequality.Semantic.DeepEqual(seed.Status.Conditions, oldSeed.Status.Conditions) {
			logger.Logger.Debugf("Seed %s conditions changed", seed.Name)
			return true
		}
	}

	if deleted {
		logger.Logger.Debugf("Seed %s deleted", seed.Name)
		return true
	}

	return false
}

// NewShootConditionsReconciler creates a reconcile.Reconciler that updates the conditions of a shoot that is registered as seed.
func NewShootConditionsReconciler(logger logrus.FieldLogger, gardenClient client.Client, cfg *config.ShootConditionsControllerConfiguration) reconcile.Reconciler {
	return &shootConditionsReconciler{
		logger:       logger,
		gardenClient: gardenClient,
		cfg:          cfg,
	}
}

type shootConditionsReconciler struct {
	logger       logrus.FieldLogger
	gardenClient client.Client
	cfg          *config.ShootConditionsControllerConfiguration
}

func (r *shootConditionsReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	shoot := &gardencorev1beta1.Shoot{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Infof("Object %q is gone, stop reconciling: %v", request.Name, err)
			return reconcile.Result{}, nil
		}
		r.logger.Infof("Unable to retrieve object %q from store: %v", request.Name, err)
		return reconcile.Result{}, err
	}

	// Get the seed this shoot is registered as
	seed, err := r.getShootSeed(ctx, shoot)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Build new shoot conditions
	// First remove all existing seed conditions and then then add the current seed conditions
	// if the shoot is still registered as seed
	seedConditionTypes := []gardencorev1beta1.ConditionType{
		gardencorev1beta1.SeedBackupBucketsReady,
		gardencorev1beta1.SeedBootstrapped,
		gardencorev1beta1.SeedExtensionsReady,
		gardencorev1beta1.SeedGardenletReady,
	}
	conditions := gardencorev1beta1helper.RemoveConditions(shoot.Status.Conditions, seedConditionTypes...)
	if seed != nil {
		conditions = gardencorev1beta1helper.MergeConditions(conditions, seed.Status.Conditions...)
	}

	// Update the shoot conditions if needed
	if gardencorev1beta1helper.ConditionsNeedUpdate(shoot.Status.Conditions, conditions) {
		r.logger.Debugf("Updating shoot %s conditions", kutil.ObjectName(shoot))
		if err := r.updateShootConditions(ctx, shoot, conditions); err != nil {
			return reconcile.Result{}, err
		}
	}

	// If the shoot is still registered as seed, requeue after the configured sync period
	if seed != nil {
		return reconcile.Result{RequeueAfter: r.cfg.SyncPeriod.Duration}, nil
	}
	return reconcile.Result{}, nil
}

func (r *shootConditionsReconciler) getShootSeed(ctx context.Context, shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Seed, error) {
	// Get the managed seed referencing this shoot
	ms, err := kutil.GetManagedSeedWithReader(ctx, r.gardenClient, shoot.Namespace, shoot.Name)
	if err != nil || ms == nil {
		return nil, err
	}

	// Get the seed registered by the managed seed
	seed := &gardencorev1beta1.Seed{}
	if err := r.gardenClient.Get(ctx, kutil.Key(ms.Name), seed); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return seed, nil
}

func (r *shootConditionsReconciler) updateShootConditions(ctx context.Context, shoot *gardencorev1beta1.Shoot, conditions []gardencorev1beta1.Condition) error {
	patch := client.StrategicMergeFrom(shoot.DeepCopy())
	shoot.Status.Conditions = conditions
	return r.gardenClient.Status().Patch(ctx, shoot, patch)
}
