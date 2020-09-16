// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"context"
	"time"

	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/resources/templates"

	"github.com/onsi/ginkgo"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	logsCount int = 10000

	initializationTimeout  = 15 * time.Minute
	getLogsFromLokiTimeout = 5 * time.Minute

	loggerDeploymentCleanupTimeout = 2 * time.Minute

	fluentBitName = "fluent-bit"
	lokiName      = "loki"
	garden        = "garden"
	logger        = "logger"
)

var _ = ginkgo.Describe("Seed logging testing", func() {

	f := framework.NewShootFramework(nil)

	framework.CBeforeEach(func(ctx context.Context) {
		checkRequiredResources(ctx, f.SeedClient)
	}, initializationTimeout)

	f.Beta().CIt("should get container logs from loki", func(ctx context.Context) {
		ginkgo.By("Calculate expected logs count")
		search, err := f.GetLokiLogs(ctx, f.ShootSeedNamespace(), logger, f.SeedClient)
		framework.ExpectNoError(err)
		expectedLogsCount := search.Data.Stats.Summary.TotalLinesProcessed + logsCount
		f.Logger.Debugf("expected logs count is %d", expectedLogsCount)

		ginkgo.By("Deploy the logger application")
		loggerParams := struct {
			HelmDeployNamespace string
			LogsCount           int
		}{
			f.ShootSeedNamespace(),
			logsCount,
		}

		err = f.RenderAndDeployTemplate(ctx, f.SeedClient, templates.LoggerAppName, loggerParams)
		framework.ExpectNoError(err)

		ginkgo.By("Wait until logger application is ready")
		loggerLabels := labels.SelectorFromSet(labels.Set(map[string]string{
			"app": logger,
		}))
		err = f.WaitUntilDeploymentsWithLabelsIsReady(ctx, loggerLabels, f.ShootSeedNamespace(), f.SeedClient)
		framework.ExpectNoError(err)

		ginkgo.By("Verify loki received logger application logs")
		err = WaitUntilLokiReceivesLogs(ctx, f, f.ShootSeedNamespace(), logger, expectedLogsCount, f.SeedClient)
		framework.ExpectNoError(err)
	}, getLogsFromLokiTimeout, framework.WithCAfterTest(func(ctx context.Context) {
		ginkgo.By("Cleaning up logger app resources")
		loggerDeploymentToDelete := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: f.ShootSeedNamespace(),
				Name:      logger,
			},
		}
		err := framework.DeleteResource(ctx, f.SeedClient, loggerDeploymentToDelete)
		framework.ExpectNoError(err)
	}, loggerDeploymentCleanupTimeout))
})
