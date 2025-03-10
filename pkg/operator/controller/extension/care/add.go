// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care

import (
	"fmt"

	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/operator/predicate"
)

// ControllerName is the name of this controller.
const ControllerName = "extension-care"

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

	_, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
			// if going into exponential backoff, wait at most the configured sync period
			RateLimiter: workqueue.NewTypedWithMaxWaitRateLimiter[reconcile.Request](
				workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](),
				r.Config.Controllers.ExtensionCare.SyncPeriod.Duration,
			),
		}).
		Watches(
			&operatorv1alpha1.Extension{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(predicate.ExtensionRequirementsChanged()),
		).Build(r)
	if err != nil {
		return err
	}

	return nil
}
