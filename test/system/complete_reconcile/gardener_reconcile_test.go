// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

/**
	Overview
		- Tests the Gardener Controller Manager reconciliation.

	Test: Shoot Reconciliation
	Expected Output
	- Should reconcile all shoots (determined if the shoot.Status.Gardener.Version == flag provided Gardener version).
 **/

package gardener_reconcile_test

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/gardener/gardener/test/framework"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils/retry"

	. "github.com/onsi/ginkgo"
)

var gardenerVersion = flag.String("version", "", "current gardener version")

const ReconcileShootsTimeout = 1 * time.Hour

func validateFlags() {
	if !framework.StringSet(*gardenerVersion) {
		Fail("you need to specify the current gardener version")
	}
}

func init() {
	framework.RegisterShootFrameworkFlags()
}

var _ = Describe("Shoot reconciliation testing", func() {

	f := framework.NewShootFramework(nil)

	framework.CIt("Should reconcile all shoots", func(ctx context.Context) {
		validateFlags()

		err := retry.UntilTimeout(ctx, 30*time.Second, ReconcileShootsTimeout, func(ctx context.Context) (bool, error) {
			shoots := &gardencorev1beta1.ShootList{}
			err := f.GardenClient.DirectClient().List(ctx, shoots)
			if err != nil {
				f.Logger.Debug(err.Error())
				return retry.MinorError(err)
			}

			reconciledShoots := 0
			for _, shoot := range shoots.Items {
				// check if the last acted gardener version is the current version,
				// to determine if the updated gardener version reconciled the shoot.
				if shoot.Status.Gardener.Version != *gardenerVersion {
					f.Logger.Debugf("last acted gardener version %s does not match current gardener version %s", shoot.Status.Gardener.Version, *gardenerVersion)
					continue
				}
				if completed, msg := framework.ShootCreationCompleted(&shoot); completed {
					reconciledShoots++
				} else {
					f.Logger.Debugf("Shoot %s not yet reconciled successfully (%s)", shoot.Name, msg)
				}

			}

			if reconciledShoots != len(shoots.Items) {
				err := fmt.Errorf("Reconciled %d of %d shoots. Waiting ...", reconciledShoots, len(shoots.Items))
				f.Logger.Info(err.Error())
				return retry.MinorError(err)
			}

			return retry.Ok()
		})
		framework.ExpectNoError(err)

	}, ReconcileShootsTimeout)

})
