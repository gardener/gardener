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

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
)

func (c *Controller) controllerInstallationOfSeedAdd(obj interface{}) {
	controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
	if !ok {
		return
	}
	c.seedExtensionCheckQueue.Add(controllerInstallation.Spec.SeedRef.Name)
}

func (c *Controller) controllerInstallationOfSeedUpdate(_, newObj interface{}) {
	c.controllerInstallationOfSeedAdd(newObj)
}

func (c *Controller) controllerInstallationOfSeedDelete(obj interface{}) {
	c.controllerInstallationOfSeedAdd(obj)
}

// NewExtensionCheckReconciler creates a new reconciler that maintains the ExtensionsReady condition of Seeds
// according to the observed changes to ControllerInstallations.
func NewExtensionCheckReconciler(clientMap clientmap.ClientMap, l logrus.FieldLogger, nowFunc func() metav1.Time) reconcile.Reconciler {
	return &extensionCheckReconciler{
		clientMap: clientMap,
		logger:    l,
		nowFunc:   nowFunc,
	}
}

type extensionCheckReconciler struct {
	clientMap clientmap.ClientMap
	logger    logrus.FieldLogger
	nowFunc   func() metav1.Time
}

func (r *extensionCheckReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := r.logger.WithField("seed", request.Name)

	gardenClient, err := r.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get garden client: %w", err)
	}

	seed := &gardencorev1beta1.Seed{}
	if err := gardenClient.Client().Get(ctx, request.NamespacedName, seed); err != nil {
		if apierrors.IsNotFound(err) {
			log.Debugf("[SEED EXTENSION CHECK] skipping because Seed has been deleted")
			return reconcile.Result{}, nil
		}
		log.Infof("[SEED EXTENSION CHECK] unable to retrieve object from store: %v", err)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, r.reconcile(ctx, gardenClient.Client(), seed)
}

func (r *extensionCheckReconciler) reconcile(ctx context.Context, gardenClient client.Client, seed *gardencorev1beta1.Seed) error {
	controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
	if err := gardenClient.List(ctx, controllerInstallationList, client.MatchingFields{core.SeedRefName: seed.Name}); err != nil {
		return err
	}

	var (
		notValid     = make(map[string]string)
		notInstalled = make(map[string]string)
		notHealthy   = make(map[string]string)
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
			requiredConditions = map[gardencorev1beta1.ConditionType]struct{}{
				gardencorev1beta1.ControllerInstallationValid:     {},
				gardencorev1beta1.ControllerInstallationInstalled: {},
				gardencorev1beta1.ControllerInstallationHealthy:   {},
			}
		)

		for _, condition := range controllerInstallation.Status.Conditions {
			if _, ok := requiredConditions[condition.Type]; !ok {
				continue
			}

			if condition.Type == gardencorev1beta1.ControllerInstallationValid && condition.Status != gardencorev1beta1.ConditionTrue {
				notValid[controllerInstallation.Name] = condition.Message
				break
			}

			if condition.Type == gardencorev1beta1.ControllerInstallationInstalled && condition.Status != gardencorev1beta1.ConditionTrue {
				notInstalled[controllerInstallation.Name] = condition.Message
				break
			}

			if condition.Type == gardencorev1beta1.ControllerInstallationHealthy && condition.Status != gardencorev1beta1.ConditionTrue {
				notHealthy[controllerInstallation.Name] = condition.Message
				break
			}

			conditionsReady++
		}

		if _, found := notHealthy[controllerInstallation.Name]; !found && conditionsReady != len(requiredConditions) {
			notHealthy[controllerInstallation.Name] = "not all required conditions found in ControllerInstallation"
		}
	}

	bldr, err := helper.NewConditionBuilder(gardencorev1beta1.SeedExtensionsReady)
	if err != nil {
		return err
	}

	if condition := helper.GetCondition(seed.Status.Conditions, gardencorev1beta1.SeedExtensionsReady); condition != nil {
		bldr.WithOldCondition(*condition)
	}

	switch {
	case len(notValid) != 0:
		bldr.
			WithStatus(gardencorev1beta1.ConditionFalse).
			WithReason("NotAllExtensionsValid").
			WithMessage(fmt.Sprintf("Some extensions are not valid: %+v", notValid))

	case len(notInstalled) != 0:
		bldr.
			WithStatus(gardencorev1beta1.ConditionFalse).
			WithReason("NotAllExtensionsInstalled").
			WithMessage(fmt.Sprintf("Some extensions are not installed: %+v", notInstalled))

	case len(notHealthy) != 0:
		bldr.
			WithStatus(gardencorev1beta1.ConditionFalse).
			WithReason("NotAllExtensionsHealthy").
			WithMessage(fmt.Sprintf("Some extensions are not healthy: %+v", notHealthy))

	default:
		bldr.
			WithStatus(gardencorev1beta1.ConditionTrue).
			WithReason("AllExtensionsReady").
			WithMessage("All extensions installed into the seed cluster are ready and healthy.")
	}

	// patch ExtensionsReady condition
	patch := client.StrategicMergeFrom(seed.DeepCopy())
	newCondition, needsUpdate := bldr.WithNowFunc(r.nowFunc).Build()
	if !needsUpdate {
		return nil
	}
	seed.Status.Conditions = helper.MergeConditions(seed.Status.Conditions, newCondition)
	return gardenClient.Status().Patch(ctx, seed, patch)
}
