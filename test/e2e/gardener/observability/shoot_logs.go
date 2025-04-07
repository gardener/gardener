// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	e2e "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/framework"
)

var parentCtx context.Context

var _ = Describe("Observability Tests", Label("Observability", "default"), func() {
	BeforeEach(func() {
		parentCtx = context.Background()
	})

	f := framework.NewShootCreationFramework(&framework.ShootCreationConfig{
		GardenerConfig: e2e.DefaultGardenConfig("garden"),
	})
	f.Shoot = e2e.DefaultShoot("e2e-observ")
	f.Shoot.Namespace = "garden"

	FIt("should create shoot & check for existing shoot logs in vali", func() {
		By("Create Shoot")
		ctx, cancel := context.WithTimeout(parentCtx, 30*time.Minute)
		defer cancel()

		Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
		f.Verify()
	})
})
