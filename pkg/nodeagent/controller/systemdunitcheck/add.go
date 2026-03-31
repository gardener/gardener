// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package systemdunitcheck

import (
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
)

// ControllerName is the name of this controller.
const ControllerName = "systemd-unit-check"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, nodePredicate predicate.Predicate) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	if r.DBus == nil {
		r.DBus = dbus.New(mgr.GetLogger().WithValues("controller", ControllerName))
	}

	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}

	if r.FS.Fs == nil {
		r.FS = afero.Afero{Fs: afero.NewOsFs()}
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
			RateLimiter:             workqueue.NewTypedWithMaxWaitRateLimiter(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](), r.Config.SyncPeriod.Duration),
			ReconciliationTimeout:   r.Config.SyncPeriod.Duration,
		}).
		For(&corev1.Node{}, builder.WithPredicates(nodePredicate, predicateutils.ForEventTypes(predicateutils.Create))).
		Complete(r)
}
