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

package controllerinstallation

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const careReconcilerName = "care"

// ManagedResourcesNamespace is the namespace where the ControllerInstallationCare controller will look for ManagedResources.
// By default the `garden` namespace is used.
// Exposed for testing.
var ManagedResourcesNamespace = v1beta1constants.GardenNamespace

func (c *Controller) controllerInstallationCareAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.controllerInstallationCareQueue.Add(key)
}

// NewCareReconciler returns an implementation of reconcile.Reconciler which is dedicated to execute care operations
func NewCareReconciler(
	gardenClient, seedClient client.Client,
	config config.ControllerInstallationCareControllerConfiguration,
) reconcile.Reconciler {
	return &careReconciler{
		gardenClient: gardenClient,
		seedClient:   seedClient,
		config:       config,
	}
}

type careReconciler struct {
	gardenClient    client.Client
	seedClient      client.Client
	config          config.ControllerInstallationCareControllerConfiguration
	gardenNamespace string
}

func (r *careReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	controllerInstallation := &gardencorev1beta1.ControllerInstallation{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, controllerInstallation); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if controllerInstallation.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}

	r.care(ctx, log, r.gardenClient, controllerInstallation)

	return reconcile.Result{RequeueAfter: r.config.SyncPeriod.Duration}, nil
}

func (r *careReconciler) care(
	ctx context.Context,
	log logr.Logger,
	gardenClient client.Client,
	controllerInstallation *gardencorev1beta1.ControllerInstallation,
) {
	// We don't return an error from this func. There is no meaningful way to handle it, because we do not want to run
	// in the exponential backoff for the condition checks.
	var (
		conditionControllerInstallationInstalled   = gardencorev1beta1helper.GetOrInitCondition(controllerInstallation.Status.Conditions, gardencorev1beta1.ControllerInstallationInstalled)
		conditionControllerInstallationHealthy     = gardencorev1beta1helper.GetOrInitCondition(controllerInstallation.Status.Conditions, gardencorev1beta1.ControllerInstallationHealthy)
		conditionControllerInstallationProgressing = gardencorev1beta1helper.GetOrInitCondition(controllerInstallation.Status.Conditions, gardencorev1beta1.ControllerInstallationProgressing)
	)

	defer func() {
		if err := patchConditions(ctx, gardenClient, controllerInstallation, conditionControllerInstallationHealthy, conditionControllerInstallationInstalled, conditionControllerInstallationProgressing); err != nil {
			log.Error(err, "Failed to patch conditions")
		}
	}()

	managedResource := &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerInstallation.Name,
			Namespace: ManagedResourcesNamespace,
		},
	}
	if err := r.seedClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource); err != nil {
		log.Error(err, "Failed to get ManagedResource", "managedResource", client.ObjectKeyFromObject(managedResource))

		msg := fmt.Sprintf("Failed to get ManagedResource %q: %s", client.ObjectKeyFromObject(managedResource).String(), err.Error())
		conditionControllerInstallationInstalled = gardencorev1beta1helper.UpdatedCondition(conditionControllerInstallationInstalled, gardencorev1beta1.ConditionUnknown, "SeedReadError", msg)
		conditionControllerInstallationHealthy = gardencorev1beta1helper.UpdatedCondition(conditionControllerInstallationHealthy, gardencorev1beta1.ConditionUnknown, "SeedReadError", msg)
		conditionControllerInstallationProgressing = gardencorev1beta1helper.UpdatedCondition(conditionControllerInstallationProgressing, gardencorev1beta1.ConditionUnknown, "SeedReadError", msg)
		return
	}

	if err := health.CheckManagedResourceApplied(managedResource); err != nil {
		conditionControllerInstallationInstalled = gardencorev1beta1helper.UpdatedCondition(conditionControllerInstallationInstalled, gardencorev1beta1.ConditionFalse, "InstallationPending", err.Error())
	} else {
		conditionControllerInstallationInstalled = gardencorev1beta1helper.UpdatedCondition(conditionControllerInstallationInstalled, gardencorev1beta1.ConditionTrue, "InstallationSuccessful", "The controller was successfully installed in the seed cluster.")
	}

	if err := health.CheckManagedResourceHealthy(managedResource); err != nil {
		conditionControllerInstallationHealthy = gardencorev1beta1helper.UpdatedCondition(conditionControllerInstallationHealthy, gardencorev1beta1.ConditionFalse, "ControllerNotHealthy", err.Error())
	} else {
		conditionControllerInstallationHealthy = gardencorev1beta1helper.UpdatedCondition(conditionControllerInstallationHealthy, gardencorev1beta1.ConditionTrue, "ControllerHealthy", "The controller running in the seed cluster is healthy.")
	}

	if err := health.CheckManagedResourceProgressing(managedResource); err != nil {
		conditionControllerInstallationProgressing = gardencorev1beta1helper.UpdatedCondition(conditionControllerInstallationProgressing, gardencorev1beta1.ConditionTrue, "ControllerNotRolledOut", err.Error())
	} else {
		conditionControllerInstallationProgressing = gardencorev1beta1helper.UpdatedCondition(conditionControllerInstallationProgressing, gardencorev1beta1.ConditionFalse, "ControllerRolledOut", "The controller has been rolled out successfully.")
	}
}
