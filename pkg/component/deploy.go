// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package component

import (
	"context"
)

// OpDestroy creates a Deployer which calls Destroy instead of Deploy.
func OpDestroy(d ...Deployer) Deployer {
	return &deploy{
		deployers: d,
		invert:    true,
		wait:      false,
	}
}

// OpWait creates a DeployWaiter which calls Wait .
func OpWait(dw ...Deployer) DeployWaiter {
	return &deploy{
		deployers: dw,
		invert:    false,
		wait:      true,
	}
}

// OpDestroyAndWait creates a DeployWaiter which calls Destroy instead of Deploy, and WaitCleanup.
func OpDestroyAndWait(dw ...Deployer) DeployWaiter {
	return &deploy{
		deployers: dw,
		invert:    true,
		wait:      true,
	}
}

// OpDestroyWithoutWait creates a DeployWaiter which calls Destroy instead of Deploy.
func OpDestroyWithoutWait(dw ...Deployer) DeployWaiter {
	return &deploy{
		deployers: dw,
		invert:    true,
		wait:      false,
	}
}

// NoOp does nothing.
func NoOp() DeployWaiter { return &deploy{} }

type deploy struct {
	deployers []Deployer
	invert    bool
	wait      bool
}

func (d *deploy) Deploy(ctx context.Context) error {
	if d.invert {
		return d.Destroy(ctx)
	}

	for _, deployer := range d.deployers {
		if deployer == nil {
			continue
		}

		if err := deployer.Deploy(ctx); err != nil {
			return err
		}

		if waiter, ok := deployer.(Waiter); ok && d.wait {
			if err := waiter.Wait(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *deploy) Destroy(ctx context.Context) error {
	for _, deployer := range d.deployers {
		if deployer == nil {
			continue
		}

		if err := deployer.Destroy(ctx); err != nil {
			return err
		}

		if waiter, ok := deployer.(Waiter); ok && d.wait {
			if err := waiter.WaitCleanup(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *deploy) Wait(ctx context.Context) error {
	if d.invert {
		return d.WaitCleanup(ctx)
	}

	for _, deployer := range d.deployers {
		if deployer == nil {
			continue
		}

		if waiter, ok := deployer.(Waiter); ok && d.wait {
			if err := waiter.Wait(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *deploy) WaitCleanup(ctx context.Context) error {
	for _, deployer := range d.deployers {
		if deployer == nil {
			continue
		}

		if waiter, ok := deployer.(Waiter); ok && d.wait {
			if err := waiter.WaitCleanup(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}
