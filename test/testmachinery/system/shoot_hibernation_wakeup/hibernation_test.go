// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoothibernationwakeup_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"

	"github.com/gardener/gardener/test/framework"
)

func init() {
	framework.RegisterShootFrameworkFlags()
}

var _ = Describe("Shoot hibernation wake-up testing", func() {
	f := framework.NewShootFramework(&framework.ShootConfig{
		SkipSeedInitialization: true,
	})

	framework.CIt("should wake up shoot", func(ctx context.Context) {
		hibernation := f.Shoot.Spec.Hibernation
		if hibernation == nil || hibernation.Enabled == nil || !*hibernation.Enabled {
			Skip("shoot is already woken up")
		}

		err := f.WakeUpShoot(ctx)
		framework.ExpectNoError(err)
	}, 30*time.Minute)
})
