// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

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
)

var _ = Describe("Garden Monitoring Tests", Label("Garden", "default"), func() {
	Describe("Prometheus health failures in Garden", Ordered, func() {
		var (
			s *GardenContext

			test = func(prometheusName string) {
				var (
					rule = &monitoringv1.PrometheusRule{
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
				)

				ItShouldCreatePrometheusRuleForGarden(s, rule)

				It("Wait until ObservabilityComponentsHealthy is false", func(ctx SpecContext) {
					Eventually(ctx, s.GardenKomega.Object(s.Garden)).Should(
						HaveField("Status.Conditions", ContainElement(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(gardencorev1beta1.ConditionType(v1beta1constants.ObservabilityComponentsHealthy)),
							"Status": Equal(gardencorev1beta1.ConditionFalse),
							"Reason": Equal("PrometheusHealthCheckDown"),
							"Message": Equal("There are health issues in Prometheus pod \"garden/prometheus-" + prometheusName + "-0\". " +
								"Access Prometheus UI and query for \"{type=\"health\"}\" for more details."),
						}))),
					)
				}, SpecTimeout(10*time.Minute))

				ItShouldDeletePrometheusRuleForGarden(s, rule)
			}
		)

		BeforeTestSetup(func() {
			backupSecret := defaultBackupSecret()
			s = NewTestContext().ForGarden(defaultGarden(backupSecret, false), backupSecret)
		})

		ItShouldCreateGarden(s)
		ItShouldWaitForGardenToBeReconciledAndHealthy(s)

		Context("Failing health checks in garden Prometheus", Ordered, func() {
			test("garden")
		})

		Context("Failing health checks in longterm Prometheus", Ordered, func() {
			test("longterm")
		})

		ItShouldDeleteGarden(s)
		ItShouldWaitForGardenToBeDeleted(s)
		ItShouldCleanUp(s)
		ItShouldWaitForExtensionToReportDeletion(s, "provider-local")
	})
})
