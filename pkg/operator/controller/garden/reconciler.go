// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package garden

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/operator/apis/config"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const finalizerName = "gardener.cloud/operator"

// Reconciler reconciles Gardens.
type Reconciler struct {
	RuntimeClient         client.Client
	RuntimeVersion        *semver.Version
	Config                config.OperatorConfiguration
	Clock                 clock.Clock
	Recorder              record.EventRecorder
	GardenNamespace       string
	ImageVector           imagevector.ImageVector
	ComponentImageVectors imagevector.ComponentImageVectors
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	garden := &operatorv1alpha1.Garden{}
	if err := r.RuntimeClient.Get(ctx, request.NamespacedName, garden); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	conditionReconciled := v1beta1helper.GetOrInitConditionWithClock(r.Clock, garden.Status.Conditions, operatorv1alpha1.GardenReconciled)
	conditionReconciled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionReconciled, gardencorev1beta1.ConditionProgressing, conditionReasonPrefix(garden)+"Progressing", "Garden operation is currently being processed.")
	if err := r.patchConditions(ctx, garden, conditionReconciled); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not patch status of %s condition to %s: %w", conditionReconciled.Type, conditionReconciled.Status, err)
	}

	secretsManager, err := secretsmanager.New(
		ctx,
		log.WithName("secretsmanager"),
		r.Clock,
		r.RuntimeClient,
		r.GardenNamespace,
		operatorv1alpha1.SecretManagerIdentityOperator,
		secretsmanager.Config{CASecretAutoRotation: true},
	)
	if err != nil {
		return reconcile.Result{}, r.patchConditionToFalse(ctx, log, garden, conditionReconciled, err)
	}

	if garden.DeletionTimestamp != nil {
		if result, err := r.delete(ctx, log, garden, secretsManager); err != nil {
			return result, r.patchConditionToFalse(ctx, log, garden, conditionReconciled, err)
		}
		return reconcile.Result{}, nil
	}

	if result, err := r.reconcile(ctx, log, garden, secretsManager); err != nil {
		return result, r.patchConditionToFalse(ctx, log, garden, conditionReconciled, err)
	}

	patch := client.MergeFrom(garden.DeepCopy())
	garden.Status.Conditions = v1beta1helper.MergeConditions(garden.Status.Conditions, v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionReconciled, gardencorev1beta1.ConditionTrue, conditionReasonPrefix(garden)+"Successful", "Garden operation was completed successfully."))
	garden.Status.ObservedGeneration = garden.Generation
	return reconcile.Result{RequeueAfter: r.Config.Controllers.Garden.SyncPeriod.Duration}, r.RuntimeClient.Status().Patch(ctx, garden, patch)
}

func (r *Reconciler) patchConditions(ctx context.Context, garden *operatorv1alpha1.Garden, condition gardencorev1beta1.Condition) error {
	patch := client.MergeFrom(garden.DeepCopy())
	garden.Status.Conditions = v1beta1helper.MergeConditions(garden.Status.Conditions, condition)
	return r.RuntimeClient.Status().Patch(ctx, garden, patch)
}

func (r *Reconciler) patchConditionToFalse(ctx context.Context, log logr.Logger, garden *operatorv1alpha1.Garden, condition gardencorev1beta1.Condition, err error) error {
	if patchErr := r.patchConditions(ctx, garden, v1beta1helper.UpdatedConditionWithClock(r.Clock, condition, gardencorev1beta1.ConditionFalse, conditionReasonPrefix(garden)+"Failed", err.Error())); err != nil {
		log.Error(patchErr, "Could not patch status", "condition", condition, "err", patchErr.Error())
	}
	return err
}

func conditionReasonPrefix(garden *operatorv1alpha1.Garden) string {
	if garden.DeletionTimestamp != nil {
		return "Deletion"
	}
	return "Reconciliation"
}

func vpaEnabled(settings *operatorv1alpha1.Settings) bool {
	if settings != nil && settings.VerticalPodAutoscaler != nil {
		return pointer.BoolDeref(settings.VerticalPodAutoscaler.Enabled, false)
	}
	return false
}
