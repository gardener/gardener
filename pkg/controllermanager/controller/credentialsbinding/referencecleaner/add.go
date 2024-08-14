// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package referencecleaner

import (
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/controllerutils"
)

// ControllerName is the name of this controller.
const ControllerName = "credentialsbinding-reference-cleaner"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Reader == nil {
		r.Reader = mgr.GetAPIReader()
	}
	if r.Writer == nil {
		r.Writer = mgr.GetClient()
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		WatchesRawSource(controllerutils.EnqueueOnce, nil).
		Complete(r)
}
