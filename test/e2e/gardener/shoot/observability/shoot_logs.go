// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	. "github.com/onsi/ginkgo/v2"
	"k8s.io/apimachinery/pkg/labels"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	. "github.com/gardener/gardener/test/e2e/gardener/shoot"
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

const (
	valiName = "vali"
)

var _ = Describe("Observability Tests", Ordered, Label("Observability", "default"), func() {
	var s *ShootContext
	BeforeTestSetup(func() {
		s = NewTestContext().ForShoot(DefaultShoot("e2e-observ"))
	})

	Describe("Create Pods to test log aggregation", Label("log-aggregation"), func() {
		ItShouldCreateShoot(s)
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

		ItShouldDeleteShoot(s)
		ItShouldWaitForShootToBeDeleted(s)
	})
})
