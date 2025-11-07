// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedseed

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	. "github.com/gardener/gardener/test/e2e/gardener/seed"
	. "github.com/gardener/gardener/test/e2e/gardener/shoot"
)

var _ = Describe("(Managed)Seed Monitoring Tests", Label("Seed", "default"), func() {
	Describe("Prometheus health failures in (Managed)Seed", Ordered, func() {
		var (
			s *ManagedSeedContext

			test = func(prometheusName string) {
				rule := &monitoringv1.PrometheusRule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      prometheusName + "-health-down",
						Namespace: "garden",
						Labels:    map[string]string{"prometheus": prometheusName},
					},
					Spec: monitoringv1.PrometheusRuleSpec{
						Groups: []monitoringv1.RuleGroup{
							{
								Name: prometheusName + "-health-down",
								Rules: []monitoringv1.Rule{
									{
										Record: "health:down",
										Expr:   intstr.FromString("vector(1)"),
										Labels: map[string]string{"type": "health"},
									},
								},
							},
						},
					},
				}

				ItShouldCreatePrometheusRuleForShoot(s.ShootContext, rule)

				It("Wait until SeedSystemComponentsHealthy is false", func(ctx SpecContext) {
					Eventually(ctx, s.GardenKomega.Object(s.SeedContext.Seed)).Should(
						HaveField("Status.Conditions", ContainElement(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(gardencorev1beta1.SeedSystemComponentsHealthy),
							"Status": Equal(gardencorev1beta1.ConditionFalse),
							"Reason": Equal("PrometheusHealthCheckDown"),
							"Message": Equal(`There are health issues in Prometheus pod "garden/prometheus-` + prometheusName + `-0". ` +
								`Access Prometheus UI and query for "{type="health"}" for more details.`),
						}))),
					)
				}, SpecTimeout(10*time.Minute))

				ItShouldDeletePrometheusRuleForShoot(s.ShootContext, rule)
			}
		)

		BeforeTestSetup(func() {
			shoot := DefaultShoot("e2e-prom-ms")
			shoot.Namespace = v1beta1constants.GardenNamespace
			managedSeed := buildManagedSeed(shoot)

			s = NewTestContext().ForManagedSeed(shoot, managedSeed)
		})

		ItShouldCreateShoot(s.ShootContext)
		ItShouldWaitForShootToBeReconciledAndHealthy(s.ShootContext)
		ItShouldCreateManagedSeed(s)
		ItShouldWaitForManagedSeedToBeReady(s)
		ItShouldWaitForSeedToBeReady(s.SeedContext)
		ItShouldInitializeManagedSeedClient(s)

		Context("Failing health checks in aggregate Prometheus", Ordered, func() {
			test("aggregate")
		})

		Context("Failing health checks in cache Prometheus", Ordered, func() {
			test("cache")
		})

		Context("Failing health checks in seed Prometheus", Ordered, func() {
			test("seed")
		})

		ItShouldDeleteManagedSeed(s)
		ItShouldWaitForSeedToBeDeleted(s.SeedContext)
		ItShouldWaitForManagedSeedToBeDeleted(s)
		ItShouldDeleteShoot(s.ShootContext)
		ItShouldWaitForShootToBeDeleted(s.ShootContext)
	})
})
