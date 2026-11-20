// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/seed"
	. "github.com/gardener/gardener/test/e2e/gardener/shoot/internal/observability"
)

var (
	valiLabels = map[string]string{
		"app":  "vali",
		"role": "logging",
	}
	randomLength   = 11
	shootLogsCount = 100
	loggerAppName  = "logger-app.yaml.tpl"
)

// ShootLoggingVali checks that the logging stack for shoots works as expected using Vali.
// It deploys two logger applications: one with Gardener labels and one without.
// It's expected that the one with Gardener labels has its logs collected in Vali by
// the log shipper (valitail). While the one without label does not.
// Function is exposed so that it can be called from the context of another
// e2e test. This is done so that we can minimize the number of shoot clusters
// that we create.
func ShootLoggingVali(tc *ShootContext) {
	itShouldSetUpShootAndSeedContext(tc)

	gardenerLoggerName := "gardener-logger" + "-" + utilrand.String(randomLength)
	nonGardenerLoggerName := "non-gardener-logger" + "-" + utilrand.String(randomLength)

	ItShouldRenderAndDeployTemplateToShootDataPlane(tc, "kube-system", loggerAppName, gardenerLoggerParams(gardenerLoggerName))
	ItShouldWaitForPodsInShootToBeReady(tc, "kube-system", labels.SelectorFromSet(map[string]string{"app": gardenerLoggerName}))

	ItShouldRenderAndDeployTemplateToShootDataPlane(tc, "kube-system", loggerAppName, nonGardenerLoggerParams(nonGardenerLoggerName))
	ItShouldWaitForPodsInShootToBeReady(tc, "kube-system", labels.SelectorFromSet(map[string]string{"app": nonGardenerLoggerName}))

	ItShouldWaitForLogsCountWithLabelToBeInVali(tc, valiLabels, "pod_name", gardenerLoggerName+".*", shootLogsCount)
	ItShouldWaitForLogsWithLabelToNotBeInVali(tc, valiLabels, "pod_name", nonGardenerLoggerName+".*")

	ItShouldWaitForLogsWithLabelToBeInVali(tc, valiLabels, "unit", "containerd.service")
	ItShouldWaitForLogsWithLabelToBeInVali(tc, valiLabels, "unit", "kubelet.service")
}

// ShootLoggingVl checks the shoot logging stack of VictoriaLogs. It deploys two loggers
// to the shoot's kube-system namespace: one with Gardener labels (expects logs in shoot
// VictoriaLogs) and one without (expects no logs in shoot VictoriaLogs). It also verifies
// shoot VictoriaLogs ingress auth and systemd unit log ingestion.
func ShootLoggingVl(tc *ShootContext) {
	itShouldSetUpShootAndSeedContext(tc)

	ItShouldHaveShootVictoriaLogsReady(tc)

	gardenerLoggerName := "gardener-logger" + "-" + utilrand.String(randomLength)
	nonGardenerLoggerName := "non-gardener-logger" + "-" + utilrand.String(randomLength)

	ItShouldRenderAndDeployTemplateToShootDataPlane(tc, "kube-system", loggerAppName, gardenerLoggerParams(gardenerLoggerName))
	ItShouldWaitForPodsInShootToBeReady(tc, "kube-system", labels.SelectorFromSet(map[string]string{"app": gardenerLoggerName}))

	ItShouldRenderAndDeployTemplateToShootDataPlane(tc, "kube-system", loggerAppName, nonGardenerLoggerParams(nonGardenerLoggerName))

	ItShouldWaitForPodsInShootToBeReady(tc, "kube-system", labels.SelectorFromSet(map[string]string{"app": nonGardenerLoggerName}))

	ItShouldRejectUnauthenticatedVictoriaLogsRequestInShoot(tc)
	ItShouldRejectWrongCredentialsVictoriaLogsRequestInShoot(tc)

	ItShouldWaitForLogsCountByKeyToBeInShootVictoriaLogs(tc, "k8s.pod.name", gardenerLoggerName+".*", time.Hour, shootLogsCount)
	ItShouldWaitForLogsByKeyNotToBeInShootVictoriaLogs(tc, "k8s.pod.name", nonGardenerLoggerName+".*", time.Hour)

	ItShouldWaitForLogsByKeyToBeInShootVictoriaLogs(tc, "unit", "containerd.service", time.Hour)
	ItShouldWaitForLogsByKeyToBeInShootVictoriaLogs(tc, "unit", "kubelet.service", time.Hour)
	ItShouldWaitForLogsByKeyToBeInShootVictoriaLogs(tc, "k8s.pod.name", "event-logger"+".*", time.Hour)
}

// SeedLoggingVl checks the seed-side routing of the shoot logging stack of VictoriaLogs.
// It deploys two logger applications. One in the shoot's control-plane namespace on the
// seed, and another in the seed's garden namespace. Both loggers are asserted to appear
// only in their respective VictoriaLogs instance and not in the other.
// Garden VictoriaLogs ingress auth is also verified.
func SeedLoggingVl(tc *ShootContext) {
	itShouldSetUpShootAndSeedContext(tc)

	ItShouldHaveShootVictoriaLogsReady(tc)
	ItShouldHaveGardenVictoriaLogsReady(tc.SeedContext)

	controlPlaneLoggerName := "seed-cp-logger" + "-" + utilrand.String(randomLength)
	gardenNamespaceLoggerName := "seed-garden-ns-logger" + "-" + utilrand.String(randomLength)

	AfterAll(func(ctx SpecContext) {
		if tc.SeedClient == nil {
			return
		}
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
			Name:      gardenNamespaceLoggerName,
			Namespace: "garden",
		}}
		Eventually(ctx, func() error {
			return client.IgnoreNotFound(tc.SeedClient.Delete(ctx, deployment))
		}).Should(Succeed())
	}, NodeTimeout(time.Minute))

	ItShouldRenderAndDeployTemplateToShootControlPlane(tc, loggerAppName, nonGardenerLoggerParams(controlPlaneLoggerName))
	ItShouldWaitForPodsInShootControlPlaneToBeReady(tc, labels.SelectorFromSet(map[string]string{"app": controlPlaneLoggerName}))

	seed.ItShouldRenderAndDeployTemplateToSeed(tc.SeedContext, "garden", loggerAppName, nonGardenerLoggerParams(gardenNamespaceLoggerName))
	seed.ItShouldWaitForPodsInGardenNamespaceToBeReady(tc.SeedContext, labels.SelectorFromSet(map[string]string{"app": gardenNamespaceLoggerName}))

	ItShouldRejectUnauthenticatedVictoriaLogsRequestInGarden(tc.SeedContext)
	ItShouldRejectWrongCredentialsVictoriaLogsRequestInGarden(tc.SeedContext)

	ItShouldWaitForLogsCountByKeyToBeInShootVictoriaLogs(tc, "k8s.pod.name", controlPlaneLoggerName+".*", time.Hour, shootLogsCount)
	ItShouldWaitForLogsByKeyNotToBeInShootVictoriaLogs(tc, "k8s.pod.name", gardenNamespaceLoggerName+".*", time.Hour)

	ItShouldWaitForLogsCountByKeyToBeInGardenVictoriaLogs(tc.SeedContext, "k8s.pod.name", gardenNamespaceLoggerName+".*", time.Hour, shootLogsCount)
	ItShouldWaitForLogsByKeyNotToBeInGardenVictoriaLogs(tc.SeedContext, "k8s.pod.name", controlPlaneLoggerName+".*", time.Hour)
}

func itShouldSetUpShootAndSeedContext(tc *ShootContext) {
	ItShouldWaitForShootToBeReconciledAndHealthy(tc)
	ItShouldInitializeShootClient(tc)
	ItShouldGetResponsibleSeed(tc)
	seed.ItShouldInitializeSeedClient(tc.SeedContext)
	ItShouldComputeControlPlaneNamespace(tc)
}

func gardenerLoggerParams(name string) map[string]any {
	return map[string]any{
		"LoggerName":   name,
		"AppLabel":     name,
		"LogsCount":    shootLogsCount,
		"LogsDuration": "20s",
		"AdditionalLabels": map[string]string{
			"origin":                              "gardener",
			"gardener.cloud/role":                 "system-component",
			"resources.gardener.cloud/managed-by": "gardener",
		},
	}
}

func nonGardenerLoggerParams(name string) map[string]any {
	return map[string]any{
		"LoggerName":   name,
		"AppLabel":     name,
		"LogsCount":    shootLogsCount,
		"LogsDuration": "20s",
	}
}
