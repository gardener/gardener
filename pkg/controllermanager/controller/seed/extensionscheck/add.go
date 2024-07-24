// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extensionscheck

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// ControllerName is the name of this controller.
const ControllerName = "seed-extensions-check"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(ctx context.Context, mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}

	// It's not possible to call builder.Build() without adding atleast one watch, and without this, we can't get the controller logger.
	// Hence, we have to build up the controller manually.
	c, err := controller.New(
		ControllerName,
		mgr,
		controller.Options{
			Reconciler:              r,
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
			// if going into exponential backoff, wait at most the configured sync period
			RateLimiter: workqueue.NewWithMaxWaitRateLimiter(workqueue.DefaultControllerRateLimiter(), r.Config.SyncPeriod.Duration),
		},
	)
	if err != nil {
		return err
	}

	return c.Watch(
		source.Kind(mgr.GetCache(),
			&gardencorev1beta1.ControllerInstallation{},
			mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper.MapFunc(r.MapControllerInstallationToSeed), mapper.UpdateWithNew, c.GetLogger()),
			predicateutils.RelevantConditionsChanged(
				func(obj client.Object) []gardencorev1beta1.Condition {
					controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
					if !ok {
						return nil
					}
					return controllerInstallation.Status.Conditions
				},
				conditionsToCheck...,
			),
		))
}

// MapControllerInstallationToSeed is a mapper.MapFunc for mapping a ControllerInstallation to the referenced Seed.
func (r *Reconciler) MapControllerInstallationToSeed(_ context.Context, _ logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
	controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
	if !ok {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: controllerInstallation.Spec.SeedRef.Name}}}
}
