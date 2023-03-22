// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	. "github.com/onsi/ginkgo/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/test/framework"
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
			err := f.GardenClient.Client().List(ctx, shoots)
			if err != nil {
				f.Logger.Error(err, "Error listing shoots")
				return retry.MinorError(err)
			}

			reconciledShoots := 0
			for _, shoot := range shoots.Items {
				log := f.Logger.WithValues("shoot", client.ObjectKeyFromObject(&shoot))

				// check if the last acted gardener version is the current version,
				// to determine if the updated gardener version reconciled the shoot.
				if shoot.Status.Gardener.Version != *gardenerVersion {
					log.Info("Last acted Gardener version does not match current Gardener version", "last", shoot.Status.Gardener.Version, "current", *gardenerVersion)
					continue
				}
				if completed, msg := framework.ShootReconciliationSuccessful(&shoot); completed {
					reconciledShoots++
				} else {
					log.Info("Shoot not yet successfully reconciled", "reason", msg)
				}
			}

			if reconciledShoots != len(shoots.Items) {
				f.Logger.Info("Reconciled shoots", "current", reconciledShoots, "desired", len(shoots.Items))
				return retry.MinorError(fmt.Errorf("reconciled %d of %d shoots, waiting", reconciledShoots, len(shoots.Items)))
			}

			return retry.Ok()
		})
		framework.ExpectNoError(err)

	}, ReconcileShootsTimeout)

})
