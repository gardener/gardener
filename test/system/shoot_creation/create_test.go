// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

/**
	Overview
		- Tests the creation of a shoot

	BeforeSuite
		- Parse Shoot from example folder and provided flags

	Test: Shoot creation
	Expected Output
		- Successful reconciliation after Shoot creation
 **/

package shoot_creation

import (
	"context"
	"time"

	"github.com/gardener/gardener/test/framework"

	. "github.com/onsi/ginkgo"
)

const (
	CreateAndReconcileTimeout = 2 * time.Hour
)

func init() {
	framework.RegisterShootCreationFrameworkFlags()
}

var _ = Describe("Shoot Creation testing", func() {

	f := framework.NewShootCreationFramework(&framework.ShootCreationConfig{
		GardenerConfig: &framework.GardenerConfig{
			CommonConfig: &framework.CommonConfig{
				ResourceDir: "../../framework/resources",
			},
		},
	})

	f.CIt("Create and Reconcile Shoot", func(ctx context.Context) {
		_, err := f.CreateShoot(ctx, true, true)
		framework.ExpectNoError(err)
	}, CreateAndReconcileTimeout)
})
