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

package seed

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/go-logr/logr"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// DefaultBackupBucketControllerName is the name of the default-backupbucket controller.
	DefaultBackupBucketControllerName = "seed-default-backupbucket"
)

func addDefaultBackupBucketController(mgr manager.Manager, config *config.SeedControllerConfiguration) error {
	reconciler := NewDefaultBackupBucketReconciler(mgr.GetLogger(), mgr.GetClient())

	ctrlOptions := controller.Options{
		Reconciler:              reconciler,
		MaxConcurrentReconciles: config.ConcurrentSyncs,
	}
	c, err := controller.New(DefaultBackupBucketControllerName, mgr, ctrlOptions)
	if err != nil {
		return err
	}

	reconciler.logger = c.GetLogger()

	backupBucket := &gardencorev1beta1.BackupBucket{}
	if err := c.Watch(&source.Kind{Type: backupBucket}, newBackupBucketEventHandler()); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", backupBucket, err)
	}

	return nil
}

func newBackupBucketEventHandler() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		// Ignore non-shoots
		bb, ok := obj.(*gardencorev1beta1.BackupBucket)
		if !ok {
			return nil
		}

		seedName := bb.Spec.SeedName
		if seedName == nil {
			return nil
		}

		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Name: *seedName,
			},
		}}
	})
}

// NewDefaultBackupBucketReconciler returns a new default control to checks backup buckets of related seeds.
func NewDefaultBackupBucketReconciler(logger logr.Logger, gardenClient client.Client) *backupBucketReconciler {
	return &backupBucketReconciler{
		logger:       logger,
		gardenClient: gardenClient,
	}
}

type backupBucketReconciler struct {
	logger       logr.Logger
	gardenClient client.Client
}

type backupBucketInfo struct {
	name     string
	errorMsg string
}

func (b *backupBucketInfo) String() string {
	return fmt.Sprintf("Name: %s, Error: %s", b.name, b.errorMsg)
}

func (r *backupBucketReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := r.logger.WithValues("backupbucket", req)

	seed := &gardencorev1beta1.Seed{}
	if err := r.gardenClient.Get(ctx, req.NamespacedName, seed); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}

		logger.Error(err, "Unable to retrieve object from store")
		return reconcile.Result{}, err
	}

	backupBucketList := &gardencorev1beta1.BackupBucketList{}
	if err := r.gardenClient.List(ctx, backupBucketList); err != nil {
		return reconcile.Result{}, err
	}

	conditionBackupBucketsReady := gardencorev1beta1helper.GetOrInitCondition(seed.Status.Conditions, gardencorev1beta1.SeedBackupBucketsReady)

	var (
		bbCount                int
		erroneousBackupBuckets []backupBucketInfo
	)
	for _, bb := range backupBucketList.Items {
		if *bb.Spec.SeedName != seed.Name {
			continue
		}

		bbCount++
		if occurred, msg := gardencorev1beta1helper.BackupBucketIsErroneous(&bb); occurred {
			erroneousBackupBuckets = append(erroneousBackupBuckets, backupBucketInfo{
				name:     bb.Name,
				errorMsg: msg,
			})
		}
	}

	switch {
	case len(erroneousBackupBuckets) > 0:
		errorMsg := "The following BackupBuckets have issues:"
		for _, bb := range erroneousBackupBuckets {
			errorMsg += fmt.Sprintf("\n* %s", bb.String())
		}

		if updateErr := patchSeedCondition(ctx, r.gardenClient, seed, gardencorev1beta1helper.UpdatedCondition(conditionBackupBucketsReady,
			gardencorev1beta1.ConditionFalse, "BackupBucketsError", errorMsg)); updateErr != nil {
			return reconcile.Result{}, updateErr
		}
	case bbCount > 0:
		if updateErr := patchSeedCondition(ctx, r.gardenClient, seed, gardencorev1beta1helper.UpdatedCondition(conditionBackupBucketsReady,
			gardencorev1beta1.ConditionTrue, "BackupBucketsAvailable", "Backup Buckets are available.")); updateErr != nil {
			return reconcile.Result{}, updateErr
		}
	case bbCount == 0:
		if updateErr := patchSeedCondition(ctx, r.gardenClient, seed, gardencorev1beta1helper.UpdatedCondition(conditionBackupBucketsReady,
			gardencorev1beta1.ConditionUnknown, "BackupBucketsGone", "Backup Buckets are gone.")); updateErr != nil {
			return reconcile.Result{}, updateErr
		}
	}

	return reconcile.Result{}, nil
}

func patchSeedCondition(ctx context.Context, c client.StatusClient, seed *gardencorev1beta1.Seed, condition gardencorev1beta1.Condition) error {
	patch := client.StrategicMergeFrom(seed.DeepCopy())

	conditions := gardencorev1beta1helper.MergeConditions(seed.Status.Conditions, condition)
	if !gardencorev1beta1helper.ConditionsNeedUpdate(seed.Status.Conditions, conditions) {
		return nil
	}

	seed.Status.Conditions = conditions
	return c.Status().Patch(ctx, seed, patch)
}
