// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

/**
	Overview
		- Tests the hibernation of a  shoot.

	Prerequisites
		- A Shoot exists.

	Test:
		Deploys a default application and hibernates the cluster.
		When the cluster is successfully hibernated it is woken up and the deployed application is tested.
	Expected Output
		- Shoot and deployed app is fully functional after hibernation and wakeup.

	Test:
		Fully reconciles a cluster which means that the default reconciliation as well as the maintenance of
		the shoot is triggered.
	Expected Output
		- Shoot is successfully reconciling.
 **/

package operations

import (
	"context"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/applications"

	"github.com/onsi/ginkgo"
)

const (
	hibernationTestTimeout = 1 * time.Hour
	reconcileTimeout       = 40 * time.Minute
)

var _ = ginkgo.Describe("Shoot operation testing", func() {

	f := framework.NewShootFramework(nil)

	f.Beta().Serial().CIt("Testing if Shoot can be hibernated successfully", func(ctx context.Context) {
		guestBookTest, err := applications.NewGuestBookTest(f)
		framework.ExpectNoError(err)

		defer guestBookTest.Cleanup(ctx)

		ginkgo.By("Deploy guestbook")
		guestBookTest.DeployGuestBookApp(ctx)
		guestBookTest.Test(ctx)

		ginkgo.By("Hibernate shoot")
		err = f.HibernateShoot(ctx)
		framework.ExpectNoError(err)

		ginkgo.By("wake up shoot")
		err = f.WakeUpShoot(ctx)
		framework.ExpectNoError(err)

		ginkgo.By("test guestbook")
		guestBookTest.WaitUntilRedisIsReady(ctx)
		guestBookTest.WaitUntilGuestbookDeploymentIsReady(ctx)
		guestBookTest.Test(ctx)

	}, hibernationTestTimeout)

	f.Beta().Serial().CIt("should fully maintain and reconcile a shoot cluster", func(ctx context.Context) {
		ginkgo.By("maintain shoot")
		err := f.UpdateShoot(ctx, func(shoot *gardencorev1beta1.Shoot) error {
			shoot.Annotations[v1beta1constants.GardenerOperation] = common.ShootOperationMaintain
			return nil
		})
		framework.ExpectNoError(err)

		ginkgo.By("reconcile shoot")
		err = f.UpdateShoot(ctx, func(shoot *gardencorev1beta1.Shoot) error {
			shoot.Annotations[v1beta1constants.GardenerOperation] = common.ShootOperationReconcile
			return nil
		})
		framework.ExpectNoError(err)
	}, reconcileTimeout)
})
