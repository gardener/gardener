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
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/logger"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (c *Controller) controllerInstallationCareAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.controllerInstallationCareQueue.Add(key)
}

func newCareReconciler(
	clientMap clientmap.ClientMap,
	l logrus.FieldLogger,
	config *config.ControllerInstallationCareControllerConfiguration,
) reconcile.Reconciler {
	return &careReconciler{
		clientMap: clientMap,
		logger:    l,
		config:    config,
	}
}

type careReconciler struct {
	clientMap clientmap.ClientMap
	logger    logrus.FieldLogger
	config    *config.ControllerInstallationCareControllerConfiguration
}

func (r *careReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logger.NewFieldLogger(r.logger, "controllerInstallation", request.Name)

	log.Debugf("[CONTROLLERINSTALLATION CARE]")
	gardenClient, err := r.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get garden client: %w", err)
	}

	controllerInstallation := &gardencorev1beta1.ControllerInstallation{}
	if err := gardenClient.Client().Get(ctx, request.NamespacedName, controllerInstallation); err != nil {
		if apierrors.IsNotFound(err) {
			log.Infof("[CONTROLLERINSTALLATION CARE] stopping care operations for ControllerInstallation since it has been deleted")
			return reconcile.Result{}, nil
		}
		log.Infof("[CONTROLLERINSTALLATION CARE] unable to retrieve object from store: %v", err)
		return reconcile.Result{}, err
	}

	if controllerInstallation.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}

	r.care(ctx, gardenClient.Client(), controllerInstallation, log)

	return reconcile.Result{RequeueAfter: r.config.SyncPeriod.Duration}, nil
}

func (r *careReconciler) care(ctx context.Context, gardenClient client.Client, controllerInstallation *gardencorev1beta1.ControllerInstallation, log logrus.FieldLogger) {
	// We don't return an error from this func. There is no meaningful way to handle it, because we do not want to run
	// in the exponential backoff for the condition checks.
	var (
		conditionControllerInstallationInstalled = gardencorev1beta1helper.GetOrInitCondition(controllerInstallation.Status.Conditions, gardencorev1beta1.ControllerInstallationInstalled)
		conditionControllerInstallationHealthy   = gardencorev1beta1helper.GetOrInitCondition(controllerInstallation.Status.Conditions, gardencorev1beta1.ControllerInstallationHealthy)
	)

	defer func() {
		if err := patchConditions(ctx, gardenClient, controllerInstallation, conditionControllerInstallationHealthy, conditionControllerInstallationInstalled); err != nil {
			log.Error("Failed to patch ControllerInstallation status: %+v", err.Error())
		}
	}()

	seedClient, err := r.clientMap.GetClient(ctx, keys.ForSeedWithName(controllerInstallation.Spec.SeedRef.Name))
	if err != nil {
		msg := fmt.Sprintf("Failed to get seed client: %s", err.Error())
		log.Error(msg)

		conditionControllerInstallationInstalled = gardencorev1beta1helper.UpdatedCondition(conditionControllerInstallationInstalled, gardencorev1beta1.ConditionUnknown, "SeedReadError", msg)
		conditionControllerInstallationHealthy = gardencorev1beta1helper.UpdatedCondition(conditionControllerInstallationHealthy, gardencorev1beta1.ConditionUnknown, "SeedReadError", msg)
		return
	}

	managedResource := &resourcesv1alpha1.ManagedResource{}
	if err := seedClient.Client().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, controllerInstallation.Name), managedResource); err != nil {
		msg := fmt.Sprintf("Failed to get ManagedResource for ControllerInstallation: %s", err.Error())
		log.Error(msg)

		conditionControllerInstallationInstalled = gardencorev1beta1helper.UpdatedCondition(conditionControllerInstallationInstalled, gardencorev1beta1.ConditionUnknown, "SeedReadError", msg)
		conditionControllerInstallationHealthy = gardencorev1beta1helper.UpdatedCondition(conditionControllerInstallationHealthy, gardencorev1beta1.ConditionUnknown, "SeedReadError", msg)
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
}
