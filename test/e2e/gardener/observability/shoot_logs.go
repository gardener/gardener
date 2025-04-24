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
	"github.com/gardener/gardener/test/utils/shoots/logging"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
)

var parentCtx context.Context

var (
	valiLabels = map[string]string{
		"app":  "vali",
		"role": "logging",
	}
	randomLength        = 11
	loggerName          = "logger"
	shootLogsCount      = 100
	shootDeltaLogsCount = 0
)

const (
	valiName = "vali"
)

var _ = Describe("Observability Tests", Label("Observability", "default"), func() {
	var (
		err error
	)

	BeforeEach(func() {
		parentCtx = context.Background()
	})

	f := framework.NewShootCreationFramework(&framework.ShootCreationConfig{
		GardenerConfig: e2e.DefaultGardenConfig("garden-local"),
	})
	f.Shoot = e2e.DefaultShoot("e2e-observ")

	FIt("check for existing shoot logs in vali from gardener pod", func() {
		var (
			shootFramework *framework.ShootFramework
		)

		ctx, cancel := context.WithTimeout(parentCtx, 20*time.Minute)
		defer cancel()

		By("Create Shoot")
		{
			Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
			f.Verify()

			shootFramework = f.ShootFramework
			// Wair for Vali to be ready
			framework.ExpectNoError(
				shootFramework.WaitUntilStatefulSetIsRunning(ctx,
					valiName, shootFramework.ShootSeedNamespace(), shootFramework.SeedClient,
				),
			)
		}
		gardenerLoggerAppLabel := "gardener-logger"
		gardenerLoggerName := "gardener-logger" + "-" + utilrand.String(randomLength)
		nonGardenerLoggerAppLabel := "non-gardener-logger"
		nonGardenerLoggerName := nonGardenerLoggerAppLabel + "-" + utilrand.String(randomLength)

		By("Create 1 pod with gardener labels")
		{
			loggerParams := map[string]any{
				"LoggerName":          gardenerLoggerName,
				"HelmDeployNamespace": "kube-system",
				"AppLabel":            gardenerLoggerAppLabel,
				"LogsCount":           100,
				"LogsDuration":        "20s",
				"AdditionalLabels": map[string]string{
					"origin":                              "gardener",
					"gardener.cloud/role":                 "system-component",
					"resources.gardener.cloud/managed-by": "gardener",
				},
			}

			err = f.RenderAndDeployTemplate(ctx, f.ShootFramework.ShootClient, templates.LoggerAppName, loggerParams)
			framework.ExpectNoError(err)
		}

		By("Create 1 pod without gardener labels")
		{
			loggerParams := map[string]any{
				"LoggerName":          nonGardenerLoggerName,
				"HelmDeployNamespace": "kube-system",
				"AppLabel":            nonGardenerLoggerAppLabel,
				"LogsCount":           100,
				"LogsDuration":        "20s",
			}

			err = f.RenderAndDeployTemplate(ctx, f.ShootFramework.ShootClient, templates.LoggerAppName, loggerParams)
			framework.ExpectNoError(err)
		}

		By("Wait for logger applications to be ready")
		{
			gardenerLoggerLabels := labels.SelectorFromSet(map[string]string{
				"app": gardenerLoggerAppLabel,
			})
			nonGardenerLoggerLabels := labels.SelectorFromSet(map[string]string{
				"app": nonGardenerLoggerAppLabel,
			})

			err = f.ShootFramework.WaitUntilDeploymentsWithLabelsIsReady(
				ctx,
				gardenerLoggerLabels,
				"kube-system",
				f.ShootFramework.ShootClient,
			)
			framework.ExpectNoError(err)
			err = f.ShootFramework.WaitUntilDeploymentsWithLabelsIsReady(
				ctx,
				nonGardenerLoggerLabels,
				"kube-system",
				f.ShootFramework.ShootClient,
			)
			framework.ExpectNoError(err)
		}

		By("Ensure that logs from the gardener pod are in Vali")
		{
			err = logging.EnsureValiLogsCount(ctx,
				30*time.Second, shootFramework, valiLabels, shootFramework.ShootSeedNamespace(),
				"pod_name", gardenerLoggerAppLabel+".*", 100, shootDeltaLogsCount, shootFramework.SeedClient,
			)
			framework.ExpectNoError(err)
		}

		By("Ensure that logs from non-gardener pod are not in Vali")
		{
			err = logging.EnsureNoValiLogs(
				ctx, shootFramework, valiLabels, shootFramework.ShootSeedNamespace(),
				"pod_name", nonGardenerLoggerAppLabel+".*", shootFramework.SeedClient,
			)
			framework.ExpectNoError(err)
		}

		By("Ensure kubelet logs exist in Vali")
		{
			err = logging.EnsureValiLogs(ctx,
				30*time.Second, shootFramework, valiLabels, shootFramework.ShootSeedNamespace(),
				"unit", "kubelet.service", shootFramework.SeedClient,
			)
		}

		By("Ensure containerd logs exist in Vali")
		{
			err = logging.EnsureValiLogs(ctx,
				30*time.Second, shootFramework, valiLabels, shootFramework.ShootSeedNamespace(),
				"unit", "containerd.service", shootFramework.SeedClient,
			)
		}

		By("Delete Shoot")
		{
			ctx, cancel = context.WithTimeout(parentCtx, 10*time.Minute)
			defer cancel()
			Expect(f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
		}
	})
})
