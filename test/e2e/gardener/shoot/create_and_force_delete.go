// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	shootextensionactuator "github.com/gardener/gardener/pkg/provider-local/controller/extension/shoot"
	e2e "github.com/gardener/gardener/test/e2e/gardener"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	test := func(shoot *gardencorev1beta1.Shoot) {
		f := defaultShootCreationFramework()
		metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, shootextensionactuator.AnnotationTestForceDeleteShoot, "true")
		f.Shoot = shoot

		It("Create and Force Delete Shoot", Label("force-delete"), func() {
			By("Create Shoot")
			ctx, cancel := context.WithTimeout(parentCtx, 30*time.Minute)
			defer cancel()
			Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
			f.Verify()

			By("Wait for Shoot to be force-deleted")
			ctx, cancel = context.WithTimeout(parentCtx, 10*time.Minute)
			defer cancel()
			Expect(f.ForceDeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
		})
	}

	Context("Shoot", func() {
		test(e2e.DefaultShoot("e2e-force-delete"))
	})

	Context("Hibernated Shoot", func() {
		shoot := e2e.DefaultShoot("e2e-fd-hib")
		shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
			Enabled: ptr.To(true),
		}

		test(shoot)
	})
})
