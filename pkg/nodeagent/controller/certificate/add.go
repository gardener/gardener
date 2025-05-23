// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certificate

import (
	"github.com/spf13/afero"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/controllerutils"
)

// ControllerName is the name of the controller.
const ControllerName = "certificate"

// AddToManager adds the lease controller with the default Options to the manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
	if r.Config == nil {
		r.Config = mgr.GetConfig()
	}
	if r.FS.Fs == nil {
		r.FS = afero.Afero{Fs: afero.NewOsFs()}
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		WatchesRawSource(controllerutils.EnqueueOnce).
		Complete(r)
}
