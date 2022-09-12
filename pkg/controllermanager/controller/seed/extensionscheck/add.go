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

package extensionscheck

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ControllerName is the name of this controller.
const ControllerName = "seed-extensions-check"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	// It's not possible to overwrite the event handler when using the controller builder. Hence, we have to build up
	// the controller manually.
	c, err := controller.New(
		ControllerName,
		mgr,
		controller.Options{
			Reconciler:              r,
			MaxConcurrentReconciles: pointer.IntDeref(r.Config.ConcurrentSyncs, 0),
			RecoverPanic:            true,
			// if going into exponential backoff, wait at most the configured sync period
			RateLimiter: workqueue.NewWithMaxWaitRateLimiter(workqueue.DefaultControllerRateLimiter(), r.Config.SyncPeriod.Duration),
		},
	)
	if err != nil {
		return err
	}

	return c.Watch(
		&source.Kind{Type: &gardencorev1beta1.ControllerInstallation{}},
		mapper.EnqueueRequestsFrom(mapper.MapFunc(r.MapControllerInstallationToSeed), mapper.UpdateWithNew, c.GetLogger()),
		r.ControllerInstallationPredicate(),
	)
}

// ControllerInstallationPredicate returns true for all events except for 'UPDATE'. Here, true is only returned when the
// status, reason or message of a relevant condition has changed.
func (r *Reconciler) ControllerInstallationPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			controllerInstallation, ok := e.ObjectNew.(*gardencorev1beta1.ControllerInstallation)
			if !ok {
				return false
			}

			oldControllerInstallation, ok := e.ObjectOld.(*gardencorev1beta1.ControllerInstallation)
			if !ok {
				return false
			}

			return shouldEnqueueControllerInstallation(oldControllerInstallation.Status.Conditions, controllerInstallation.Status.Conditions)
		},
	}
}

func shouldEnqueueControllerInstallation(oldConditions, newConditions []gardencorev1beta1.Condition) bool {
	for _, condition := range conditionsToCheck {
		if wasConditionStatusReasonOrMessageUpdated(
			gardencorev1beta1helper.GetCondition(oldConditions, condition),
			gardencorev1beta1helper.GetCondition(newConditions, condition),
		) {
			return true
		}
	}

	return false
}

func wasConditionStatusReasonOrMessageUpdated(oldCondition, newCondition *gardencorev1beta1.Condition) bool {
	return (oldCondition == nil && newCondition != nil) ||
		(oldCondition != nil && newCondition == nil) ||
		(oldCondition != nil && newCondition != nil &&
			(oldCondition.Status != newCondition.Status || oldCondition.Reason != newCondition.Reason || oldCondition.Message != newCondition.Message))
}

// MapControllerInstallationToSeed is a mapper.MapFunc for mapping a ControllerInstallation to the referenced Seed.
func (r *Reconciler) MapControllerInstallationToSeed(_ context.Context, _ logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
	controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
	if !ok {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: controllerInstallation.Spec.SeedRef.Name}}}
}
