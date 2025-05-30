// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"k8s.io/apimachinery/pkg/labels"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	. "github.com/gardener/gardener/test/e2e/gardener"
	. "github.com/gardener/gardener/test/e2e/gardener/shoot/internal/observability"
	"github.com/gardener/gardener/test/framework/resources/templates"
)

var (
	valiLabels = map[string]string{
		"app":  "vali",
		"role": "logging",
	}
	randomLength   = 11
	shootLogsCount = 100
)

// ShootLogging checks that the logging stack for shoots works as expected.
// It deploys two logger applications: one with Gardener labels and one without.
// It's expected that the one with Gardener labels has its logs collected in Vali by
// the log shipper (valitail). While the one without label does not.
// Function is exposed so that it can be called from the context of another
// e2e test. This is done so that we can minimize the number of shoot clusters
// that we create.
func ShootLogging(s *ShootContext) {
	ItShouldWaitForShootToBeReconciledAndHealthy(s)
	ItShouldInitializeShootClient(s)
	ItShouldGetResponsibleSeed(s)
	ItShouldInitializeSeedClient(s)
	ItShouldComputeControlPlaneNamespace(s)

	gardenerLoggerAppLabel := "gardener-logger"
	gardenerLoggerName := "gardener-logger" + "-" + utilrand.String(randomLength)
	nonGardenerLoggerAppLabel := "non-gardener-logger"
	nonGardenerLoggerName := nonGardenerLoggerAppLabel + "-" + utilrand.String(randomLength)

	loggerParams := map[string]any{
		"LoggerName":          gardenerLoggerName,
		"HelmDeployNamespace": "kube-system",
		"AppLabel":            gardenerLoggerAppLabel,
		"LogsCount":           shootLogsCount,
		"LogsDuration":        "20s",
		"AdditionalLabels": map[string]string{
			"origin":                              "gardener",
			"gardener.cloud/role":                 "system-component",
			"resources.gardener.cloud/managed-by": "gardener",
		},
	}
	ItShouldRenderAndDeployTemplateToShoot(s, templates.LoggerAppName, loggerParams)

	loggerParams = map[string]any{
		"LoggerName":          nonGardenerLoggerName,
		"HelmDeployNamespace": "kube-system",
		"AppLabel":            nonGardenerLoggerAppLabel,
		"LogsCount":           shootLogsCount,
		"LogsDuration":        "20s",
	}
	ItShouldRenderAndDeployTemplateToShoot(s, templates.LoggerAppName, loggerParams)

	gardenerLoggerLabels := labels.SelectorFromSet(map[string]string{
		"app": gardenerLoggerAppLabel,
	})
	ItShouldWaitForPodsInShootToBeReady(s, "kube-system", gardenerLoggerLabels)

	nonGardenerLoggerLabels := labels.SelectorFromSet(map[string]string{
		"app": nonGardenerLoggerAppLabel,
	})
	ItShouldWaitForPodsInShootToBeReady(s, "kube-system", nonGardenerLoggerLabels)

	ItShouldWaitForLogsCountWithLabelToBeInVali(s, valiLabels, "pod_name", gardenerLoggerAppLabel+".*", shootLogsCount)
	ItShouldWaitForLogsWithLabelToNotBeInVali(s, valiLabels, "pod_name", nonGardenerLoggerAppLabel+".*")

	ItShouldWaitForLogsWithLabelToBeInVali(s, valiLabels, "unit", "containerd.service")
	ItShouldWaitForLogsWithLabelToBeInVali(s, valiLabels, "unit", "kubelet.service")
}
