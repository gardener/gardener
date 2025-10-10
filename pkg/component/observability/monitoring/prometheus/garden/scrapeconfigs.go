// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	_ "embed"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/aggregate"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

type federationConfig struct {
	name      string
	targets   []monitoringv1alpha1.Target
	isIngress bool
	secret    *corev1.Secret
}

//go:embed assets/scrapeconfigs/cadvisor.yaml
var cAdvisor string

// AdditionalScrapeConfigs returns the additional scrape configs for the garden prometheus.
func AdditionalScrapeConfigs() []string {
	return []string{cAdvisor}
}

// CentralScrapeConfigs returns the central ScrapeConfig resources for the garden prometheus.
func CentralScrapeConfigs(prometheusAggregateTargets []monitoringv1alpha1.Target, prometheusAggregateIngressTargets []monitoringv1alpha1.Target, globalMonitoringSecret *corev1.Secret) []*monitoringv1alpha1.ScrapeConfig {
	out := []*monitoringv1alpha1.ScrapeConfig{{
		ObjectMeta: metav1.ObjectMeta{
			Name: "prometheus",
		},
		Spec: monitoringv1alpha1.ScrapeConfigSpec{
			StaticConfigs: []monitoringv1alpha1.StaticConfig{{
				Targets: []monitoringv1alpha1.Target{"localhost:9090"},
			}},
			RelabelConfigs: []monitoringv1.RelabelConfig{{
				Action:      "replace",
				Replacement: ptr.To("prometheus-garden"),
				TargetLabel: "job",
			}},
			MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig("prometheus_(.+)"),
		},
	}}

	if len(prometheusAggregateTargets) > 0 {
		name := "prometheus-" + aggregate.Label
		out = append(out, newScrapeConfigForFederation(federationConfig{name: name, targets: prometheusAggregateTargets, isIngress: false}))
	}

	if len(prometheusAggregateIngressTargets) > 0 && globalMonitoringSecret != nil {
		name := "prometheus-" + aggregate.Label + "-ingress"
		out = append(out, newScrapeConfigForFederation(federationConfig{name: name, targets: prometheusAggregateIngressTargets, isIngress: true, secret: globalMonitoringSecret}))
	}

	return out
}

func newScrapeConfigForFederation(federation federationConfig) *monitoringv1alpha1.ScrapeConfig {
	config := &monitoringv1alpha1.ScrapeConfig{
		ObjectMeta: metav1.ObjectMeta{Name: federation.name},
		Spec: monitoringv1alpha1.ScrapeConfigSpec{
			HonorLabels:     ptr.To(true),
			HonorTimestamps: ptr.To(false),
			MetricsPath:     ptr.To("/federate"),
			Params: map[string][]string{
				"match[]": {
					`{__name__=~"seed:(.+):count"}`,
					`{__name__=~"seed:(.+):sum"}`,
					`{__name__=~"seed:(.+):sum_cp"}`,
					`{__name__=~"seed:(.+):sum_by_pod",namespace=~"extension-(.+)"}`,
					`{__name__=~"seed:(.+):sum_by_container",__name__!="seed:kube_pod_container_status_restarts_total:sum_by_container",container="kube-apiserver"}`,
					`{__name__=~"shoot:(.+):(.+)",__name__!="shoot:apiserver_storage_objects:sum_by_resource",__name__!="shoot:apiserver_watch_duration:quantile"}`,
					`{__name__="ALERTS"}`,
					`{__name__="shoot:availability"}`,
					`{__name__="prometheus_tsdb_lowest_timestamp"}`,
					`{__name__="prometheus_tsdb_storage_blocks_bytes"}`,
					`{__name__="seed:persistentvolume:inconsistent_size"}`,
					`{__name__="seed:kube_pod_container_status_restarts_total:max_by_namespace"}`,
					`{__name__=~"metering:.+:(sum_by_namespace|sum_by_instance_type)"}`,
					`{__name__="kube_node_spec_taint"}`,
				},
			},
			StaticConfigs: []monitoringv1alpha1.StaticConfig{{Targets: federation.targets}},
			RelabelConfigs: []monitoringv1.RelabelConfig{{
				Action:      "replace",
				Replacement: ptr.To("prometheus-" + aggregate.Label),
				TargetLabel: "job",
			}},
			MetricRelabelConfigs: []monitoringv1.RelabelConfig{{
				SourceLabels: []monitoringv1.LabelName{"alertname"},
				TargetLabel:  "shoot_alertname",
			}},
		},
	}

	if federation.isIngress {
		config.Spec.Scheme = ptr.To("HTTPS")
		config.Spec.TLSConfig = &monitoringv1.SafeTLSConfig{InsecureSkipVerify: ptr.To(true)}
		config.Spec.BasicAuth = &monitoringv1.BasicAuth{
			Username: corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: federation.secret.Name}, Key: secretsutils.DataKeyUserName},
			Password: corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: federation.secret.Name}, Key: secretsutils.DataKeyPassword},
		}
	}

	return config
}
