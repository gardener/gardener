// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package dnsrecord

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/controller/dnsrecord"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

var (
	// DefaultAddOptions are the default AddOptions for AddToManager.
	DefaultAddOptions = AddOptions{}
)

// AddOptions are options to apply when adding the local dnsrecord controller to the manager.
type AddOptions struct {
	// Controller are the controller.Options.
	Controller controller.Options
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation or not.
	IgnoreOperationAnnotation bool
}

// AddToManagerWithOptions adds a controller with the given Options to the given manager.
// The opts.Reconciler is being set with a newly instantiated actuator.
func AddToManagerWithOptions(ctx context.Context, mgr manager.Manager, opts AddOptions) error {
	return dnsrecord.Add(ctx, mgr, dnsrecord.AddArgs{
		Actuator:          NewActuator(mgr),
		ControllerOptions: opts.Controller,
		Predicates:        dnsrecord.DefaultPredicates(ctx, mgr, opts.IgnoreOperationAnnotation),
		Type:              local.Type,
	})
}

// AddToManager adds a controller with the default Options.
func AddToManager(ctx context.Context, mgr manager.Manager) error {
	return AddToManagerWithOptions(ctx, mgr, DefaultAddOptions)
}
