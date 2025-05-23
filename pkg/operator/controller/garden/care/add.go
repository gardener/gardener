// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	"github.com/gardener/gardener/pkg/operator/predicate"
)

// ControllerName is the name of this controller.
const ControllerName = "garden-care"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, gardenClientMap clientmap.ClientMap) error {
	if r.RuntimeClient == nil {
		r.RuntimeClient = mgr.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
	if r.GardenNamespace == "" {
		r.GardenNamespace = v1beta1constants.GardenNamespace
	}
	if gardenClientMap == nil {
		return fmt.Errorf("gardenClientMap must not be nil")
	}
	r.GardenClientMap = gardenClientMap

	c, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
			// if going into exponential backoff, wait at most the configured sync period
			RateLimiter: workqueue.NewTypedWithMaxWaitRateLimiter(
				workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](),
				r.Config.Controllers.GardenCare.SyncPeriod.Duration,
			),
		}).
		Watches(
			&operatorv1alpha1.Garden{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(predicate.GardenCreatedOrReconciledSuccessfully()),
		).Build(r)
	if err != nil {
		return err
	}

	r.registerManagedResourceWatchFunc = func() error {
		return c.Watch(source.Kind[client.Object](
			mgr.GetCache(),
			&resourcesv1alpha1.ManagedResource{},
			handler.EnqueueRequestsFromMapFunc(r.MapManagedResourceToGarden(mgr.GetLogger().WithValues("controller", ControllerName))),
			predicateutils.ManagedResourceConditionsChanged(),
		))
	}

	return nil
}

// MapManagedResourceToGarden is a handler.MapFunc for mapping a ManagedResource to the owning Garden.
func (r *Reconciler) MapManagedResourceToGarden(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, _ client.Object) []reconcile.Request {
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
}
