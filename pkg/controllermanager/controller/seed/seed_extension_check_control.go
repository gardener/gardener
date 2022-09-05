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

package seed

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	controllermanagerconfig "github.com/gardener/gardener/pkg/controllermanager/apis/config"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const extensionCheckReconcilerName = "extension-check"

var conditionsToCheck = []gardencorev1beta1.ConditionType{
	gardencorev1beta1.ControllerInstallationValid,
	gardencorev1beta1.ControllerInstallationInstalled,
	gardencorev1beta1.ControllerInstallationHealthy,
	gardencorev1beta1.ControllerInstallationProgressing,
}

func (c *Controller) controllerInstallationOfSeedAdd(obj interface{}) {
	controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
	if !ok {
		return
	}
	c.seedExtensionsCheckQueue.Add(controllerInstallation.Spec.SeedRef.Name)
}

func (c *Controller) controllerInstallationOfSeedUpdate(oldObj, newObj interface{}) {
	oldControllerInstallation, ok1 := oldObj.(*gardencorev1beta1.ControllerInstallation)
	newControllerInstallation, ok2 := newObj.(*gardencorev1beta1.ControllerInstallation)
	if !ok1 || !ok2 {
		return
	}

	if shouldEnqueueControllerInstallation(oldControllerInstallation.Status.Conditions, newControllerInstallation.Status.Conditions) {
		c.controllerInstallationOfSeedAdd(newObj)
	}
}

func (c *Controller) controllerInstallationOfSeedDelete(obj interface{}) {
	c.controllerInstallationOfSeedAdd(obj)
}

// NewExtensionsCheckReconciler creates a new reconciler that maintains the ExtensionsReady condition of Seeds
// according to the observed changes to ControllerInstallations.
func NewExtensionsCheckReconciler(
	gardenClient client.Client,
	config controllermanagerconfig.SeedExtensionsCheckControllerConfiguration,
	clock clock.Clock,
) reconcile.Reconciler {
	return &extensionCheckReconciler{
		gardenClient: gardenClient,
		config:       config,
		clock:        clock,
	}
}

type extensionCheckReconciler struct {
	gardenClient client.Client
	config       controllermanagerconfig.SeedExtensionsCheckControllerConfiguration
	clock        clock.Clock
}

func (r *extensionCheckReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	seed := &gardencorev1beta1.Seed{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, seed); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}
	if err := r.reconcile(ctx, seed); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: r.config.SyncPeriod.Duration}, nil
}

func (r *extensionCheckReconciler) reconcile(ctx context.Context, seed *gardencorev1beta1.Seed) error {
	controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
	if err := r.gardenClient.List(ctx, controllerInstallationList, client.MatchingFields{core.SeedRefName: seed.Name}); err != nil {
		return err
	}

	var (
		notValid     = make(map[string]string)
		notInstalled = make(map[string]string)
		notHealthy   = make(map[string]string)
		progressing  = make(map[string]string)
	)

	for _, controllerInstallation := range controllerInstallationList.Items {
		// not needed for real client, but fake client doesn't support field selector
		// see https://github.com/kubernetes-sigs/controller-runtime/issues/1376
		// could be solved by switching from fake client to real client against envtest
		if controllerInstallation.Spec.SeedRef.Name != seed.Name {
			continue
		}

		if len(controllerInstallation.Status.Conditions) == 0 {
			notInstalled[controllerInstallation.Name] = "extension was not yet installed"
			continue
		}

		var (
			conditionsReady    = 0
			requiredConditions = map[gardencorev1beta1.ConditionType]struct{}{}
		)

		for _, condition := range conditionsToCheck {
			requiredConditions[condition] = struct{}{}
		}

		for _, condition := range controllerInstallation.Status.Conditions {
			if _, ok := requiredConditions[condition.Type]; !ok {
				continue
			}

			switch {
			case condition.Type == gardencorev1beta1.ControllerInstallationValid && condition.Status != gardencorev1beta1.ConditionTrue:
				notValid[controllerInstallation.Name] = condition.Message
			case condition.Type == gardencorev1beta1.ControllerInstallationInstalled && condition.Status != gardencorev1beta1.ConditionTrue:
				notInstalled[controllerInstallation.Name] = condition.Message
			case condition.Type == gardencorev1beta1.ControllerInstallationHealthy && condition.Status != gardencorev1beta1.ConditionTrue:
				notHealthy[controllerInstallation.Name] = condition.Message
			case condition.Type == gardencorev1beta1.ControllerInstallationProgressing && condition.Status != gardencorev1beta1.ConditionFalse:
				progressing[controllerInstallation.Name] = condition.Message
			}

			conditionsReady++
		}

		if _, found := notHealthy[controllerInstallation.Name]; !found && conditionsReady != len(requiredConditions) {
			notHealthy[controllerInstallation.Name] = "not all required conditions found in ControllerInstallation"
		}
	}

	condition := helper.GetOrInitCondition(seed.Status.Conditions, gardencorev1beta1.SeedExtensionsReady)
	extensionsReadyThreshold := getThresholdForCondition(r.config.ConditionThresholds, gardencorev1beta1.SeedExtensionsReady)

	switch {
	case len(notValid) != 0:
		condition = setToProgressingOrFalse(r.clock, extensionsReadyThreshold, condition, "NotAllExtensionsValid", fmt.Sprintf("Some extensions are not valid: %+v", notValid))
	case len(notInstalled) != 0:
		condition = setToProgressingOrFalse(r.clock, extensionsReadyThreshold, condition, "NotAllExtensionsInstalled", fmt.Sprintf("Some extensions are not installed: %+v", notInstalled))
	case len(notHealthy) != 0:
		condition = setToProgressingOrFalse(r.clock, extensionsReadyThreshold, condition, "NotAllExtensionsHealthy", fmt.Sprintf("Some extensions are not healthy: %+v", notHealthy))
	case len(progressing) != 0:
		condition = setToProgressingOrFalse(r.clock, extensionsReadyThreshold, condition, "SomeExtensionsProgressing", fmt.Sprintf("Some extensions are progressing: %+v", progressing))
	default:
		condition = helper.UpdatedCondition(condition, gardencorev1beta1.ConditionTrue, "AllExtensionsReady", "All extensions installed into the seed cluster are ready and healthy.")
	}

	return patchSeedCondition(ctx, r.gardenClient, seed, condition)
}

func shouldEnqueueControllerInstallation(oldConditions, newConditions []gardencorev1beta1.Condition) bool {
	for _, condition := range conditionsToCheck {
		oldCondition := gardencorev1beta1helper.GetCondition(oldConditions, condition)
		newCondition := gardencorev1beta1helper.GetCondition(newConditions, condition)
		if wasConditionStatusReasonOrMessageUpdated(oldCondition, newCondition) {
			return true
		}
	}

	return false
}

func wasConditionStatusReasonOrMessageUpdated(oldCondition, newCondition *gardencorev1beta1.Condition) bool {
	return oldCondition == nil && newCondition != nil ||
		oldCondition != nil && newCondition == nil ||
		oldCondition != nil && newCondition != nil &&
			(oldCondition.Status != newCondition.Status || oldCondition.Reason != newCondition.Reason || oldCondition.Message != newCondition.Message)
}
