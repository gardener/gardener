// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/labels"

	e2e "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/resources/templates"
)

var parentCtx context.Context

var _ = Describe("Observability Tests", Label("Observability", "default"), func() {
	BeforeEach(func() {
		parentCtx = context.Background()
	})

	f := framework.NewShootCreationFramework(&framework.ShootCreationConfig{
		GardenerConfig: e2e.DefaultGardenConfig("garden-local"),
	})
	f.Shoot = e2e.DefaultShoot("e2e-observ")

	FIt("should create shoot & check for existing shoot logs in vali", func() {
		By("Create Shoot")
		ctx, cancel := context.WithTimeout(parentCtx, 30*time.Minute)
		defer cancel()

		Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
		f.Verify()

		By("Creating a pod with gardener labels and waiting for it to be healthy")
		loggerParams := map[string]any{
			"LoggerName":          "cool-logger",
			"HelmDeployNamespace": "kube-system",
			"AppLabel":            "logger",
			"LogsCount":           100,
			"LogsDuration":        "20s",
		}

		err := f.RenderAndDeployTemplate(ctx, f.ShootFramework.ShootClient, templates.LoggerAppName, loggerParams)
		framework.ExpectNoError(err)

		loggerLabels := labels.SelectorFromSet(map[string]string{
			"app": "logger",
		})

		err = f.ShootFramework.WaitUntilDeploymentsWithLabelsIsReady(
			ctx,
			loggerLabels,
			"kube-system",
			f.ShootFramework.ShootClient,
		)
		framework.ExpectNoError(err)

	})
})
