// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/seed"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/inclusterclient"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/node"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	Describe("Create, Hibernate, Wake up and Delete Shoot", func() {
		test := func(tc *ShootContext, testPrometheusHealthCheck bool) {
			BeforeAll(func() {
				tc.Init()
			})

			ItShouldCreateShoot(tc)
			ItShouldWaitForShootToBeReconciledAndHealthy(tc)
			ItShouldInitializeShootClient(tc)
			ItShouldGetResponsibleSeed(tc)
			seed.ItShouldInitializeSeedClient(tc.SeedContext)

			// validate Prometheus health checks are in place for the shoot Prometheus.
			if testPrometheusHealthCheck {
				itShouldVerifyShootPrometheusHealthCheck(tc)
			}

			if !v1beta1helper.IsWorkerless(tc.Shoot) {
				inclusterclient.VerifyInClusterAccessToAPIServer(tc)

				// We verify the node readiness feature in this specific e2e test because it uses a single-node shoot cluster.
				// The default shoot e2e test deals with multiple nodes, deleting all of them and waiting for them to be recreated
				// might increase the test duration undesirably.
				node.VerifyNodeCriticalComponentsBootstrapping(tc)
			}

			ItShouldHibernateShoot(tc)
			ItShouldWaitForShootToBeReconciledAndHealthy(tc)

			ItShouldWakeUpShoot(tc)
			ItShouldWaitForShootToBeReconciledAndHealthy(tc)

			if !v1beta1helper.IsWorkerless(tc.Shoot) {
				inclusterclient.VerifyInClusterAccessToAPIServer(tc)
			}

			ItShouldDeleteShoot(tc)
			ItShouldWaitForShootToBeDeleted(tc)
		}

		Context("Shoot with workers", Label("basic"), Ordered, PriorityLong, func() {
			test(NewShootContext(DefaultShoot("e2e-wake-up")), true)
		})

		Context("Workerless Shoot", Label("workerless"), Ordered, PriorityLong, func() {
			test(NewShootContext(DefaultWorkerlessShoot("e2e-wake-up")), false)
		})
	})
})

func itShouldVerifyShootPrometheusHealthCheck(tc *ShootContext) {
	if os.Getenv("IPFAMILY") == "ipv6" {
		// TODO(vicwicker): Run the tests normally for IPv6 once the shoot cross-node communication in the local setup works.
		tc.Log.Info("Skip shoot Prometheus health check test in IPv6 mode due to cross-node communication issues in the test setup")
		return
	}

	rule := &monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shoot-test-job-down",
			Namespace: "shoot--local--" + tc.Shoot.Name,
			Labels:    map[string]string{"prometheus": "shoot"},
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{
				{
					Name: "shoot-test-job-down",
					Rules: []monitoringv1.Rule{
						{
							Record: "up",
							Expr:   intstr.FromString("vector(0)"),
							Labels: map[string]string{"job": "test"},
						},
					},
				},
			},
		},
	}

	seed.ItShouldCreatePrometheusRuleForSeed(tc.SeedContext, rule)

	It("Wait until ObservabilityComponentsHealthy is false", func(ctx SpecContext) {
		Eventually(ctx, tc.GardenKomega.Object(tc.Shoot)).Should(
			HaveField("Status.Conditions", ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(gardencorev1beta1.ShootObservabilityComponentsHealthy),
				"Status": Equal(gardencorev1beta1.ConditionFalse),
				"Reason": Equal("PrometheusHealthCheckDown"),
				"Message": Equal(`There are health issues in Prometheus pod "shoot--local--` + tc.Shoot.Name + `/prometheus-shoot-0". ` +
					`Access Prometheus UI and query for "healthcheck:up" for more details: healthcheck:up{job="test", task="target:down"} => 0`),
			}))),
		)
	}, SpecTimeout(10*time.Minute))

	seed.ItShouldDeletePrometheusRuleForSeed(tc.SeedContext, rule)

	It("Wait until ObservabilityComponentsHealthy is true", func(ctx SpecContext) {
		Eventually(ctx, tc.GardenKomega.Object(tc.Shoot)).Should(
			HaveField("Status.Conditions", ContainElement(MatchFields(IgnoreExtras, Fields{
				"Type":    Equal(gardencorev1beta1.ShootObservabilityComponentsHealthy),
				"Status":  Equal(gardencorev1beta1.ConditionTrue),
				"Reason":  Equal("ObservabilityComponentsRunning"),
				"Message": Equal("All observability components are healthy."),
			}))),
		)
	}, SpecTimeout(10*time.Minute))
}
