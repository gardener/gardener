// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	"context"
	"fmt"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	e2e "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/resources/templates"
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

//	func getLogCountFromResult(search *framework.SearchResponse) (int, error) {
//		var totalLogs int
//		for _, result := range search.Data.Result {
//			currentStr, ok := result.Value[1].(string)
//			if !ok {
//				return totalLogs, fmt.Errorf("Data.Result.Value[1] is not a string")
//			}
//			current, err := strconv.Atoi(currentStr)
//			if err != nil {
//				return totalLogs, fmt.Errorf("Data.Result.Value[1] string is not parsable to integer")
//			}
//			totalLogs += current
//		}
//		return totalLogs, nil
//	}
//
// This was copied from test/testmachinery/shoots/logging/utils.go
// Vali labels variable was removed from function signature.
// WaitUntilValiReceivesLogs waits until the vali instance in <valiNamespace> receives <expected> logs for <key>=<value>
func WaitUntilValiReceivesLogs(ctx context.Context, interval time.Duration, shootFramework *framework.ShootFramework, valiNamespace, key, value string, expected, delta int, c kubernetes.Interface) error {
	err := retry.Until(ctx, interval, func(ctx context.Context) (done bool, err error) {
		search, err := shootFramework.GetValiLogs(ctx, valiLabels, valiNamespace, key, value, c)
		if err != nil {
			return retry.SevereError(err)
		}
		var actual int
		for _, result := range search.Data.Result {
			currentStr, ok := result.Value[1].(string)
			if !ok {
				return retry.SevereError(fmt.Errorf("Data.Result.Value[1] is not a string for %s=%s", key, value))
			}
			current, err := strconv.Atoi(currentStr)
			if err != nil {
				return retry.SevereError(fmt.Errorf("Data.Result.Value[1] string is not parsable to integer for %s=%s", key, value))
			}
			actual += current
		}

		log := shootFramework.Logger.WithValues("expected", expected, "actual", actual)

		if expected > actual {
			log.Info("Waiting to receive all expected logs")
			return retry.MinorError(fmt.Errorf("received only %d/%d logs", actual, expected))
		} else if expected+delta < actual {
			return retry.SevereError(fmt.Errorf("expected to receive %d logs but was %d", expected, actual))
		}

		log.Info("Received logs", "delta", delta)
		return retry.Ok()
	})

	if err != nil {
		// ctx might have been cancelled already, make sure we still dump logs, so use context.Background()
		dumpLogsCtx, dumpLogsCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer dumpLogsCancel()

		shootFramework.Logger.Info("Dump Vali logs")
		if dumpError := shootFramework.DumpLogsForPodInNamespace(dumpLogsCtx, c, valiNamespace, "vali-0",
			&corev1.PodLogOptions{Container: "vali"}); dumpError != nil {
			shootFramework.Logger.Error(dumpError, "Error dumping logs for pod")
		}

		shootFramework.Logger.Info("Dump Fluent-bit logs")
		labels := client.MatchingLabels{"app": "fluent-bit"}
		if dumpError := shootFramework.DumpLogsForPodsWithLabelsInNamespace(dumpLogsCtx, c, "garden",
			labels); dumpError != nil {
			shootFramework.Logger.Error(dumpError, "Error dumping logs for pod")
		}
	}

	return err
}

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

			// Wait for gardener logger to be ready
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
			err = WaitUntilValiReceivesLogs(ctx,
				30*time.Second, shootFramework, shootFramework.ShootSeedNamespace(),
				"pod_name", gardenerLoggerAppLabel+".*", 100, shootDeltaLogsCount, shootFramework.SeedClient,
			)
			framework.ExpectNoError(err)
		}

		By("Ensure that logs from non-gardener pod are not in Vali")
		{
		}

		By("Delete Shoot")
		{
			ctx, cancel = context.WithTimeout(parentCtx, 10*time.Minute)
			defer cancel()
			Expect(f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
		}
	})

	FIt("should create shoot & assure there are no logs in vali from non-gardener pod", func() {
		// By("Create Shoot")
		// ctx, cancel := context.WithTimeout(parentCtx, 30*time.Minute)
		// defer cancel()
		//
		// Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
		// f.Verify()
		//
		// shootFramework := f.ShootFramework
		// fullLoggerName := loggerName + "-" + utilrand.String(randomLength)
		// loggerRegex := fullLoggerName + "-.*"
		//
		// By("Wait until Vali StatefulSet is ready")
		// framework.ExpectNoError(
		// 	shootFramework.WaitUntilStatefulSetIsRunning(ctx,
		// 		valiName, shootFramework.ShootSeedNamespace(), shootFramework.SeedClient,
		// 	),
		// )
		//
		// By("Get logs in Vali till now")
		// search, err := shootFramework.GetValiLogs(ctx,
		// 	valiLabels, shootFramework.ShootSeedNamespace(), "pod_name",
		// 	loggerRegex, shootFramework.SeedClient,
		// )
		// framework.ExpectNoError(err)
		// initialLogsCount, err := getLogCountFromResult(search)
		// framework.ExpectNoError(err)
		// expectedLogs := initialLogsCount

		// By("Creating a pod with gardener labels and waiting for it to be healthy")
		// loggerParams := map[string]any{
		// 	"LoggerName":          "cool-logger",
		// 	"HelmDeployNamespace": "kube-system",
		// 	"AppLabel":            "logger",
		// 	"LogsCount":           100,
		// 	"LogsDuration":        "20s",
		// }
		//
		// err = f.RenderAndDeployTemplate(ctx, f.ShootFramework.ShootClient, templates.LoggerAppName, loggerParams)
		// framework.ExpectNoError(err)
		//
		// loggerLabels := labels.SelectorFromSet(map[string]string{
		// 	"app": "logger",
		// })
		//
		// err = f.ShootFramework.WaitUntilDeploymentsWithLabelsIsReady(
		// 	ctx,
		// 	loggerLabels,
		// 	"kube-system",
		// 	f.ShootFramework.ShootClient,
		// )
		// framework.ExpectNoError(err)

		// By("Check that the correct number of logs present in Vali")
		// err = WaitUntilValiReceivesLogs(ctx,
		// 	30*time.Second, shootFramework, shootFramework.ShootSeedNamespace(),
		// 	"pod_name", loggerRegex, expectedLogs, shootDeltaLogsCount, shootFramework.SeedClient,
		// )
		// framework.ExpectNoError(err)

		By("Ensure that logs from the non-gardener pod are NOT in Vali")
	})
})
