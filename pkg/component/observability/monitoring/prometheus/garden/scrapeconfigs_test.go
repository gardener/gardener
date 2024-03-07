// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package garden_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/garden"
)

var _ = Describe("PrometheusRules", func() {
	Describe("#AdditionalScrapeConfigs", func() {
		It("should return the expected objects", func() {
			Expect(garden.AdditionalScrapeConfigs()).To(HaveExactElements(
				`job_name: cadvisor
honor_labels: false
scheme: https

tls_config:
  ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token

kubernetes_sd_configs:
- role: node

relabel_configs:
- source_labels: [__meta_kubernetes_node_address_InternalIP]
  target_label: instance
- action: labelmap
  regex: __meta_kubernetes_node_label_(.+)
- target_label: __address__
  replacement: kubernetes.default.svc
- source_labels: [__meta_kubernetes_node_name]
  regex: (.+)
  target_label: __metrics_path__
  replacement: /api/v1/nodes/${1}/proxy/metrics/cadvisor

metric_relabel_configs:
- source_labels: [ namespace ]
  regex: garden
  action: keep
- source_labels: [ __name__ ]
  regex: ^(container_cpu_usage_seconds_total|container_fs_reads_bytes_total|container_fs_writes_bytes_total|container_fs_inodes_total|container_fs_limit_bytes|container_fs_usage_bytes|container_last_seen|container_memory_working_set_bytes|container_network_receive_bytes_total|container_network_transmit_bytes_total)$
  action: keep
`,
			))
		})
	})

	Describe("#CentralScrapeConfigs", func() {
		When("no global monitoring secret provided", func() {
			It("should only contain the prometheus-garden scrape config", func() {
				Expect(garden.CentralScrapeConfigs(nil, nil)).To(HaveExactElements(&monitoringv1alpha1.ScrapeConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "prometheus"},
					Spec: monitoringv1alpha1.ScrapeConfigSpec{
						StaticConfigs: []monitoringv1alpha1.StaticConfig{{
							Targets: []monitoringv1alpha1.Target{"localhost:9090"},
						}},
						RelabelConfigs: []*monitoringv1.RelabelConfig{{
							Action:      "replace",
							Replacement: "prometheus-garden",
							TargetLabel: "job",
						}},
						MetricRelabelConfigs: []*monitoringv1.RelabelConfig{{
							SourceLabels: []monitoringv1.LabelName{"__name__"},
							Action:       "keep",
							Regex:        `^(prometheus_(.+))$`,
						}},
					},
				}))
			})
		})

		When("global monitoring secret provided", func() {
			var (
				prometheusAggregateTargets = []monitoringv1alpha1.Target{"foo", "bar"}
				globalMonitoringSecret     = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "global-monitoring-secret"}}
			)

			It("should also contain the aggregate prometheus scrape config", func() {
				Expect(garden.CentralScrapeConfigs(prometheusAggregateTargets, globalMonitoringSecret)).To(ContainElement(&monitoringv1alpha1.ScrapeConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "prometheus-aggregate"},
					Spec: monitoringv1alpha1.ScrapeConfigSpec{
						HonorLabels:     ptr.To(true),
						HonorTimestamps: ptr.To(false),
						MetricsPath:     ptr.To("/federate"),
						Scheme:          ptr.To("HTTPS"),
						Params: map[string][]string{
							"match[]": {
								`{__name__=~"seed:(.+):count"}`,
								`{__name__=~"seed:(.+):sum"}`,
								`{__name__=~"seed:(.+):sum_cp"}`,
								`{__name__=~"seed:(.+):sum_by_pod",namespace=~"extension-(.+)"}`,
								`{__name__=~"shoot:(.+):(.+)",__name__!="shoot:apiserver_storage_objects:sum_by_resource",__name__!="shoot:apiserver_watch_duration:quantile"}`,
								`{__name__="ALERTS"}`,
								`{__name__="shoot:availability"}`,
								`{__name__="prometheus_tsdb_lowest_timestamp"}`,
								`{__name__="prometheus_tsdb_storage_blocks_bytes"}`,
								`{__name__="seed:persistentvolume:inconsistent_size"}`,
								`{__name__="seed:kube_pod_container_status_restarts_total:max_by_namespace"}`,
								`{__name__=~"metering:.+:(sum_by_namespace|sum_by_instance_type)"}`,
							},
						},
						TLSConfig: &monitoringv1.SafeTLSConfig{InsecureSkipVerify: true},
						BasicAuth: &monitoringv1.BasicAuth{
							Username: corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: globalMonitoringSecret.Name}, Key: "username"},
							Password: corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: globalMonitoringSecret.Name}, Key: "password"},
						},
						StaticConfigs: []monitoringv1alpha1.StaticConfig{{Targets: prometheusAggregateTargets}},
						RelabelConfigs: []*monitoringv1.RelabelConfig{{
							Action:      "replace",
							Replacement: "prometheus-aggregate",
							TargetLabel: "job",
						}},
						MetricRelabelConfigs: []*monitoringv1.RelabelConfig{{
							SourceLabels: []monitoringv1.LabelName{"alertname"},
							TargetLabel:  "shoot_alertname",
						}},
					},
				}))
			})
		})
	})
})
