// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certmanagement

import (
	"context"

	"github.com/gardener/gardener/pkg/component"
)

var _ component.DeployWaiter = &destroyer{}

type destroyer struct {
	deployer component.DeployWaiter
}

// NewDestroyer wraps a deployer and calls its Destroy method on Deploy.
func NewDestroyer(deployer component.DeployWaiter) component.DeployWaiter {
	return &destroyer{deployer: deployer}
}

func (d *destroyer) Deploy(ctx context.Context) error {
	return d.deployer.Destroy(ctx)
}

func (d *destroyer) Destroy(ctx context.Context) error {
	return d.deployer.Destroy(ctx)
}

func (d *destroyer) Wait(ctx context.Context) error {
	return d.deployer.WaitCleanup(ctx)
}

func (d *destroyer) WaitCleanup(ctx context.Context) error {
	return d.deployer.WaitCleanup(ctx)
}
