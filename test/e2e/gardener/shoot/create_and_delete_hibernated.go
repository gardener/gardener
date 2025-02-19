// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	e2e "github.com/gardener/gardener/test/e2e/gardener"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	test := func(shoot *gardencorev1beta1.Shoot) {
		f := defaultShootCreationFramework()
		f.Shoot = shoot

		f.Shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
			Enabled: ptr.To(true),
		}

		It("Create and Delete Hibernated Shoot", Offset(1), Label("hibernated"), func() {
			By("Create Shoot")
			ctx, cancel := context.WithTimeout(parentCtx, 30*time.Minute)
			defer cancel()
			Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
			f.Verify()

			By("Verify no running pods")
			Expect(f.GardenerFramework.VerifyNoRunningPods(ctx, f.Shoot)).To(Succeed())

			By("Delete Shoot")
			ctx, cancel = context.WithTimeout(parentCtx, 15*time.Minute)
			defer cancel()
			Expect(f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
		})
	}

	Context("Shoot with workers", func() {
		test(e2e.DefaultShoot("e2e-hib"))
	})

	Context("Workerless Shoot", Label("workerless"), func() {
		test(e2e.DefaultWorkerlessShoot("e2e-hib"))
	})
})
