// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package heartbeat

import (
	"context"

	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/pkg/controllerutils"
)

// ControllerName is the name of the controller.
const ControllerName = "heartbeat"

// DefaultAddOptions are the default AddOptions for AddToManager.
var DefaultAddOptions = AddOptions{
	RenewIntervalSeconds: 30,
}

// AddOptions are options to apply when adding the heartbeat controller to the manager.
type AddOptions struct {
	// ExtensionName is the name of the extension that this heartbeat controller is part of.
	ExtensionName string
	// Namespace is the namespace which will be used for the heartbeat lease resource.
	Namespace string
	// RenewIntervalSeconds defines how often the heartbeat lease is renewed.
	RenewIntervalSeconds int32
}

// AddToManager adds the heartbeat controller with the default Options to the manager.
func AddToManager(_ context.Context, mgr manager.Manager) error {
	return Add(mgr, AddArgs{
		ExtensionName:        DefaultAddOptions.ExtensionName,
		Namespace:            DefaultAddOptions.Namespace,
		RenewIntervalSeconds: DefaultAddOptions.RenewIntervalSeconds,
		Clock:                clock.RealClock{},
	})
}

// AddArgs are arguments for adding a heartbeat controller to a manager.
type AddArgs struct {
	// ControllerOptions are the controller.Options.
	ControllerOptions controller.Options
	// ExtensionName is the name of the extension controller.
	ExtensionName string
	// Namespace is the namespace which will be used for the heartbeat lease resource.
	Namespace string
	// RenewIntervalSeconds defines how often the heartbeat lease is renewed.
	RenewIntervalSeconds int32
	// Clock is the clock to use when renewing the heartbeat lease resource.
	Clock clock.Clock
}

// Add creates a new heartbeat controller and adds it to the given manager.
func Add(mgr manager.Manager, args AddArgs) error {
	args.ControllerOptions.MaxConcurrentReconciles = 1

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(args.ControllerOptions).
		WatchesRawSource(controllerutils.EnqueueOnce).
		Complete(NewReconciler(mgr, args.ExtensionName, args.Namespace, args.RenewIntervalSeconds, args.Clock))
}
