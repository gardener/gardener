// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package lease

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// ControllerName is the name of the controller.
const ControllerName = "lease"

// AddToManager adds the lease controller with the default Options to the manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, nodePredicate predicate.Predicate) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.LeaseDurationSeconds == 0 {
		r.LeaseDurationSeconds = 40
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
	if r.Namespace == "" {
		r.Namespace = metav1.NamespaceSystem
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&corev1.Node{}, builder.WithPredicates(nodePredicate, predicateutils.ForEventTypes(predicateutils.Create))).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}
