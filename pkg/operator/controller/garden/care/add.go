// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
	clientmapbuilder "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/builder"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	"github.com/gardener/gardener/pkg/operator/predicate"
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
			&operatorv1alpha1.Garden{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(predicate.GardenPredicate()),
		).Build(r)
	if err != nil {
		return err
	}

	r.registerManagedResourceWatchFunc = func() error {
		return c.Watch(
			source.Kind(mgr.GetCache(), &resourcesv1alpha1.ManagedResource{}),
			mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper.MapFunc(r.MapManagedResourceToGarden), mapper.UpdateWithNew, c.GetLogger()),
			predicateutils.ManagedResourceConditionsChanged(),
		)
	}

	return nil
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
