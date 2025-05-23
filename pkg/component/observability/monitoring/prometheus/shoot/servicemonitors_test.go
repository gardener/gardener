// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
)

var _ = Describe("ServiceMonitors", func() {
	Describe("#CentralServiceMonitors", func() {
		When("alertmanager is wanted", func() {
			Expect(shoot.CentralServiceMonitors(true)).To(HaveExactElements(&monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{Name: "alertmanager-shoot"},
				Spec: monitoringv1.ServiceMonitorSpec{
					Selector: metav1.LabelSelector{MatchLabels: map[string]string{
						"component":    "alertmanager",
						"role":         "monitoring",
						"alertmanager": "shoot",
					}},
					Endpoints: []monitoringv1.Endpoint{{
						Port: "metrics",
						RelabelConfigs: []monitoringv1.RelabelConfig{{
							Action: "labelmap",
							Regex:  `__meta_kubernetes_service_label_(.+)`,
						}},
						MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
							SourceLabels: []monitoringv1.LabelName{"__name__"},
							Action:       "keep",
							Regex:        `^(alertmanager_config_hash|alertmanager_config_last_reload_successful|process_max_fds|process_open_fds)$`,
						}},
					}},
				},
			}))
		})

		When("alertmanager is not wanted", func() {
			It("should return the expected objects", func() {
				Expect(shoot.CentralServiceMonitors(false)).To(BeEmpty())
			})
		})
	})
})
