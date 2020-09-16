// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package component

import "context"

// OpDestroy creates a DeployWaiter which calls Destroy instead of create
// and WaitCleanup instead of Wait
func OpDestroy(dw ...DeployWaiter) DeployWaiter {
	return &deploy{
		dw:          dw,
		invert:      true,
		destroyOnly: false,
	}
}

// OpDestroyAndWait creates a Deployer which calls Destroy instead of create
// and waits for destruction.
func OpDestroyAndWait(dw ...DeployWaiter) Deployer {
	return &deploy{
		dw:          dw,
		invert:      true,
		destroyOnly: true,
	}
}

// OpWaiter creates a Deployer which calls waits for each operation.
func OpWaiter(dw ...DeployWaiter) Deployer {
	return &deploy{
		dw:          dw,
		invert:      false,
		destroyOnly: true,
	}
}

// NoOp does nothing
func NoOp() DeployWaiter { return &deploy{} }

type deploy struct {
	invert      bool
	destroyOnly bool
	dw          []DeployWaiter
}

func (d *deploy) Deploy(ctx context.Context) error {
	if d.invert {
		return d.Destroy(ctx)
	}

	for _, dw := range d.dw {
		if dw == nil {
			continue
		}

		if err := dw.Deploy(ctx); err != nil {
			return err
		}

		if d.destroyOnly {
			if err := dw.Wait(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *deploy) Destroy(ctx context.Context) error {
	for _, dw := range d.dw {
		if dw == nil {
			continue
		}

		if err := dw.Destroy(ctx); err != nil {
			return err
		}

		if d.destroyOnly {
			if err := dw.WaitCleanup(ctx); err != nil {
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

	for _, dw := range d.dw {
		if dw == nil {
			continue
		}

		if err := dw.Wait(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (d *deploy) WaitCleanup(ctx context.Context) error {
	for _, dw := range d.dw {
		if dw == nil {
			continue
		}

		if err := dw.WaitCleanup(ctx); err != nil {
			return err
		}
	}

	return nil
}
