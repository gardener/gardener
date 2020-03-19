// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
