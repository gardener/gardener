// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerutils

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// ControlledRunner is a Runnable for the controller-runtime manager which can be used to control complex start-up
// sequences of controllers. It allows to first run a set of bootstrap runnables before adding the actual runnables to
// the manager. When the manager is started, this runner first runs all bootstrapping runnables before adding the actual
// runnables to the manager.
type ControlledRunner struct {
	// Manager is the controller-runtime manager.
	Manager manager.Manager
	// BootstrapRunnables are the runnables that are responsible for bootstrapping tasks. They will be started
	// sequentially in the provided order.
	BootstrapRunnables []manager.Runnable
	// ActualRunnables are the runnables that are responsible for the actual tasks of the controller. They will be added
	// sequentially in the provided order, however they will be started immediately if the manager is already started.
	ActualRunnables []manager.Runnable
}

// Start starts the runner.
func (c *ControlledRunner) Start(ctx context.Context) error {
	for _, runnable := range c.BootstrapRunnables {
		if err := runnable.Start(ctx); err != nil {
			return fmt.Errorf("failed during bootstrapping: %w", err)
		}
	}

	return AddAllRunnables(c.Manager, c.ActualRunnables...)
}

// AddAllRunnables loops over the provided runnables and adds them to the manager. It returns an error immediately if
// adding fails.
func AddAllRunnables(mgr manager.Manager, runnables ...manager.Runnable) error {
	for _, r := range runnables {
		if err := mgr.Add(r); err != nil {
			return fmt.Errorf("failed adding runnable to manager: %w", err)
		}
	}

	return nil
}
