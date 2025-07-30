// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dns

import (
	"cmp"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/third_party/mock/controller-runtime/predicate"
)

// ControllerName is the name of this controller.
const ControllerName = "seed-dns"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	r.GardenNamespace = cmp.Or(r.GardenNamespace, v1beta1constants.GardenNamespace)

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&gardencorev1beta1.Seed{}, builder.WithPredicates(r.SeedPredicate())).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 0,
		}).
		Complete(r)
}

// SeedPredicate returns true for all events for which the seed does not have spec.dns.internal set.
func (r *Reconciler) SeedPredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		seed, ok := obj.(*gardencorev1beta1.Seed)
		if !ok {
			return false
		}

		return seed.Spec.DNS.Internal == nil
	})
}
