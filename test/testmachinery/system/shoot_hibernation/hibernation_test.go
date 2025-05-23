// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoothibernation_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"

	"github.com/gardener/gardener/test/framework"
)

func init() {
	framework.RegisterShootFrameworkFlags()
}

var _ = Describe("Shoot hibernation testing", func() {
	f := framework.NewShootFramework(&framework.ShootConfig{
		SkipSeedInitialization: true,
	})

	framework.CIt("should hibernate shoot", func(ctx context.Context) {
		hibernation := f.Shoot.Spec.Hibernation
		if hibernation != nil && hibernation.Enabled != nil && *hibernation.Enabled {
			Skip("shoot is already hibernated")
		}

		err := f.HibernateShoot(ctx)
		framework.ExpectNoError(err)
	}, 30*time.Minute)
})
