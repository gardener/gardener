// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	. "github.com/gardener/gardener/test/e2e/gardener/seed"
)

var _ = Describe("Shoot Monitoring Tests", Label("Shoot", "default"), func() {
	Describe("Prometheus health failures in Shoot", Ordered, func() {
		var (
			s *ShootContext

			rule = &monitoringv1.PrometheusRule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-health-down",
					Namespace: "shoot--local--e2e-prom",
					Labels:    map[string]string{"prometheus": "shoot"},
				},
				Spec: monitoringv1.PrometheusRuleSpec{
					Groups: []monitoringv1.RuleGroup{
						{
							Name: "shoot-health-down",
							Rules: []monitoringv1.Rule{
								{
									Record: "healthcheck",
									Expr:   intstr.FromString("vector(1)"),
									Labels: map[string]string{"task": "test"},
								},
							},
						},
					},
				},
			}
		)

		BeforeTestSetup(func() {
			s = NewTestContext().ForShoot(DefaultShoot("e2e-prom"))
		})

		ItShouldCreateShoot(s)
		ItShouldWaitForShootToBeReconciledAndHealthy(s)
		ItShouldGetResponsibleSeed(s)
		ItShouldInitializeSeedClient(&s.SeedContext)

		Context("Failing health checks in shoot Prometheus", Ordered, func() {
			ItShouldCreatePrometheusRuleForSeed(&s.SeedContext, rule)

			It("Wait until ObservabilityComponentsHealthy is false", func(ctx SpecContext) {
				Eventually(ctx, s.GardenKomega.Object(s.Shoot)).Should(
					HaveField("Status.Conditions", ContainElement(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(gardencorev1beta1.ShootObservabilityComponentsHealthy),
						"Status": Equal(gardencorev1beta1.ConditionFalse),
						"Reason": Equal("PrometheusHealthCheckDown"),
						"Message": Equal("There are health issues in Prometheus pod \"shoot--local--e2e-prom/prometheus-shoot-0\". " +
							"Access Prometheus UI and query for \"healthcheck:alert\" for more details."),
					}))),
				)
			}, SpecTimeout(10*time.Minute))

			ItShouldDeletePrometheusRuleForSeed(&s.SeedContext, rule)
		})

		ItShouldDeleteShoot(s)
		ItShouldWaitForShootToBeDeleted(s)
	})
})
