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

// OpDestroy creates a Deployer which calls Destroy instead of Deploy.
func OpDestroy(d ...Deployer) Deployer {
	return &deploy{
		deployers: d,
		invert:    true,
		wait:      false,
	}
}

// OpWait creates a DeployWaiter which calls Wait .
func OpWait(dw ...DeployWaiter) DeployWaiter {
	return &deploy{
		deployWaiters: dw,
		invert:        false,
		wait:          true,
	}
}

// OpDestroyAndWait creates a DeployWaiter which calls Destroy instead of Deploy, and WaitCleanup.
func OpDestroyAndWait(dw ...DeployWaiter) DeployWaiter {
	return &deploy{
		deployWaiters: dw,
		invert:        true,
		wait:          true,
	}
}

// OpDestroyWithoutWait creates a DeployWaiter which calls Destroy instead of Deploy.
func OpDestroyWithoutWait(dw ...DeployWaiter) DeployWaiter {
	return &deploy{
		deployWaiters: dw,
		invert:        true,
		wait:          false,
	}
}

// NoOp does nothing.
func NoOp() DeployWaiter { return &deploy{} }

type deploy struct {
	deployers     []Deployer
	deployWaiters []DeployWaiter
	invert        bool
	wait          bool
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
	}

	for _, deployWaiter := range d.deployWaiters {
		if deployWaiter == nil {
			continue
		}

		if err := deployWaiter.Deploy(ctx); err != nil {
			return err
		}

		if d.wait {
			if err := deployWaiter.Wait(ctx); err != nil {
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
	}

	for _, deployWaiter := range d.deployWaiters {
		if deployWaiter == nil {
			continue
		}

		if err := deployWaiter.Destroy(ctx); err != nil {
			return err
		}

		if d.wait {
			if err := deployWaiter.WaitCleanup(ctx); err != nil {
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

	for _, deployWaiter := range d.deployWaiters {
		if deployWaiter == nil {
			continue
		}

		if d.wait {
			if err := deployWaiter.Wait(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *deploy) WaitCleanup(ctx context.Context) error {
	for _, deployWaiter := range d.deployWaiters {
		if deployWaiter == nil {
			continue
		}

		if d.wait {
			if err := deployWaiter.WaitCleanup(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}
