// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package care

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	clientmapbuilder "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/builder"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// ControllerName is the name of this controller.
const ControllerName = "garden-care"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(ctx context.Context, mgr manager.Manager) error {
	if r.RuntimeClient == nil {
		r.RuntimeClient = mgr.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
	if r.GardenNamespace == "" {
		r.GardenNamespace = v1beta1constants.GardenNamespace
	}
	if r.GardenClientMap == nil {
		var err error
		r.GardenClientMap, err = clientmapbuilder.
			NewGardenClientMapBuilder().
			WithRuntimeClient(mgr.GetClient()).
			WithClientConnectionConfig(&r.Config.VirtualClientConnection).
			WithGardenNamespace(r.GardenNamespace).
			Build(mgr.GetLogger())
		if err != nil {
			return fmt.Errorf("failed to build garden ClientMap: %w", err)
		}
		if err := mgr.Add(r.GardenClientMap); err != nil {
			return err
		}
	}

	c, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
			// if going into exponential backoff, wait at most the configured sync period
			RateLimiter: workqueue.NewWithMaxWaitRateLimiter(
				workqueue.DefaultControllerRateLimiter(),
				r.Config.Controllers.GardenCare.SyncPeriod.Duration,
			),
		}).
		Watches(
			source.NewKindWithCache(&operatorv1alpha1.Garden{}, mgr.GetCache()),
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(r.GardenPredicate()),
		).Build(r)
	if err != nil {
		return err
	}

	return c.Watch(
		source.NewKindWithCache(&resourcesv1alpha1.ManagedResource{}, mgr.GetCache()),
		mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper.MapFunc(r.MapManagedResourceToGarden), mapper.UpdateWithNew, c.GetLogger()),
		predicateutils.ManagedResourceConditionsChanged(),
	)
}

// GardenPredicate is a predicate which returns 'true' for create events, and for update events in case the garden was
// successfully reconciled.
func (r *Reconciler) GardenPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			garden, ok := e.ObjectNew.(*operatorv1alpha1.Garden)
			if !ok {
				return false
			}

			oldGarden, ok := e.ObjectOld.(*operatorv1alpha1.Garden)
			if !ok {
				return false
			}

			// re-evaluate health status right after a reconciliation operation has succeeded
			return predicateutils.ReconciliationFinishedSuccessfully(oldGarden.Status.LastOperation, garden.Status.LastOperation)
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

// MapManagedResourceToGarden is a mapper.MapFunc for mapping a ManagedResource to the owning Garden.
func (r *Reconciler) MapManagedResourceToGarden(ctx context.Context, log logr.Logger, _ client.Reader, _ client.Object) []reconcile.Request {
	gardenList := &operatorv1alpha1.GardenList{}
	if err := r.RuntimeClient.List(ctx, gardenList, client.Limit(1)); err != nil {
		log.Error(err, "Could not list gardens")
		return nil
	}

	if len(gardenList.Items) == 0 {
		return nil
	}
	garden := gardenList.Items[0]

	// A garden reconciliation typically touches most of the existing ManagedResources and this will cause the
	// ManagedResource controller to frequently change their conditions. In this case, we don't want to spam the API
	// server with updates on the Garden conditions.
	if garden.Status.LastOperation != nil && garden.Status.LastOperation.State == gardencorev1beta1.LastOperationStateProcessing {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: garden.Name}}}
}
