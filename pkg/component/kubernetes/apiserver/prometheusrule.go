// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"context"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/controllerutils"
)

func (k *kubeAPIServer) emptyPrometheusRule() *monitoringv1.PrometheusRule {
	return &monitoringv1.PrometheusRule{ObjectMeta: monitoringutils.ConfigObjectMeta(k.values.NamePrefix+v1beta1constants.DeploymentNameKubeAPIServer, k.namespace, k.prometheusLabel())}
}

func (k *kubeAPIServer) reconcilePrometheusRule(ctx context.Context, prometheusRule *monitoringv1.PrometheusRule) error {
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client.Client(), prometheusRule, func() error {
		prometheusRule.Spec = monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{{
				Name: "kube-apiserver.rules",
				Rules: []monitoringv1.Rule{
					{
						Alert: "ApiServerNotReachable",
						Expr:  intstr.FromString(`probe_success{job="blackbox-apiserver"} == 0`),
						For:   ptr.To(monitoringv1.Duration("5m")),
						Labels: map[string]string{
							"service":    v1beta1constants.DeploymentNameKubeAPIServer,
							"severity":   "blocker",
							"type":       "seed",
							"visibility": "all",
						},
						Annotations: map[string]string{
							"summary":     "API server not reachable (externally).",
							"description": "API server not reachable via external endpoint: {{ $labels.instance }}.",
						},
					},
					{
						Alert: "KubeApiserverDown",
						Expr:  intstr.FromString(`absent(up{job="kube-apiserver"} == 1)`),
						For:   ptr.To(monitoringv1.Duration("5m")),
						Labels: map[string]string{
							"service":    v1beta1constants.DeploymentNameKubeAPIServer,
							"severity":   "blocker",
							"type":       "seed",
							"visibility": "operator",
						},
						Annotations: map[string]string{
							"summary":     "API server unreachable.",
							"description": "All API server replicas are down/unreachable, or all API server could not be found.",
						},
					},
					{
						Alert: "KubeApiServerTooManyOpenFileDescriptors",
						Expr:  intstr.FromString(`100 * process_open_fds{job="kube-apiserver"} / process_max_fds > 50`),
						For:   ptr.To(monitoringv1.Duration("30m")),
						Labels: map[string]string{
							"service":    v1beta1constants.DeploymentNameKubeAPIServer,
							"severity":   "warning",
							"type":       "seed",
							"visibility": "owner",
						},
						Annotations: map[string]string{
							"summary":     "The API server has too many open file descriptors",
							"description": "The API server ({{ $labels.instance }}) is using {{ $value }}% of the available file/socket descriptors.",
						},
					},
					{
						Alert: "KubeApiServerTooManyOpenFileDescriptors",
						Expr:  intstr.FromString(`100 * process_open_fds{job="kube-apiserver"} / process_max_fds{job="kube-apiserver"} > 80`),
						For:   ptr.To(monitoringv1.Duration("30m")),
						Labels: map[string]string{
							"service":    v1beta1constants.DeploymentNameKubeAPIServer,
							"severity":   "critical",
							"type":       "seed",
							"visibility": "owner",
						},
						Annotations: map[string]string{
							"summary":     "The API server has too many open file descriptors",
							"description": "The API server ({{ $labels.instance }}) is using {{ $value }}% of the available file/socket descriptors.",
						},
					},
					{
						Alert: "KubeApiServerLatency",
						// Some verbs excluded because they are expected to be long-lasting:
						// - WATCHLIST is long-poll
						// - CONNECT is "kubectl exec"
						Expr: intstr.FromString(`histogram_quantile(0.99, sum without (instance,resource,subresource) (rate(apiserver_request_duration_seconds_bucket{subresource!~"log|portforward|exec|proxy|attach",verb!~"CONNECT|WATCHLIST|WATCH|PROXY proxy"}[5m]))) > 3`),
						For:  ptr.To(monitoringv1.Duration("30m")),
						Labels: map[string]string{
							"service":    v1beta1constants.DeploymentNameKubeAPIServer,
							"severity":   "warning",
							"type":       "seed",
							"visibility": "owner",
						},
						Annotations: map[string]string{
							"summary":     "Kubernetes API server latency is high",
							"description": "Kube API server latency for verb {{ $labels.verb }} is high. This could be because the shoot workers and the control plane are in different regions. 99th percentile of request latency is greater than 3 seconds.",
						},
					},
					{
						Record: "shoot:apiserver_watch_duration:quantile",
						Expr:   intstr.FromString(`histogram_quantile(0.2, sum(rate(apiserver_request_duration_seconds_bucket{verb="WATCH",resource=~"configmaps|deployments|secrets|daemonsets|services|nodes|pods|namespaces|endpoints|statefulsets|clusterroles|roles"}[5m])) by (le,scope,resource))`),
						Labels: map[string]string{"quantile": "0.2"},
					},
					{
						Record: "shoot:apiserver_watch_duration:quantile",
						Expr:   intstr.FromString(`histogram_quantile(0.5, sum(rate(apiserver_request_duration_seconds_bucket{verb="WATCH",resource=~"configmaps|deployments|secrets|daemonsets|services|nodes|pods|namespaces|endpoints|statefulsets|clusterroles|roles"}[5m])) by (le,scope,resource))`),
						Labels: map[string]string{"quantile": "0.5"},
					},
					{
						Record: "shoot:apiserver_watch_duration:quantile",
						Expr:   intstr.FromString(`histogram_quantile(0.9, sum(rate(apiserver_request_duration_seconds_bucket{verb="WATCH",resource=~"configmaps|deployments|secrets|daemonsets|services|nodes|pods|namespaces|endpoints|statefulsets|clusterroles|roles"}[5m])) by (le,scope,resource))`),
						Labels: map[string]string{"quantile": "0.9"},
					},
					{
						Record: "shoot:apiserver_watch_duration:quantile",
						Expr:   intstr.FromString(`histogram_quantile(0.2, sum(rate(apiserver_request_duration_seconds_bucket{verb="WATCH",group=~".+garden.+"}[5m])) by (le,scope,resource))`),
						Labels: map[string]string{"quantile": "0.2"},
					},
					{
						Record: "shoot:apiserver_watch_duration:quantile",
						Expr:   intstr.FromString(`histogram_quantile(0.5, sum(rate(apiserver_request_duration_seconds_bucket{verb="WATCH",group=~".+garden.+"}[5m])) by (le,scope,resource))`),
						Labels: map[string]string{"quantile": "0.5"},
					},
					{
						Record: "shoot:apiserver_watch_duration:quantile",
						Expr:   intstr.FromString(`histogram_quantile(0.9, sum(rate(apiserver_request_duration_seconds_bucket{verb="WATCH",group=~".+garden.+"}[5m])) by (le,scope,resource))`),
						Labels: map[string]string{"quantile": "0.9"},
					},

					// API Auditlog
					{
						Alert: "KubeApiServerTooManyAuditlogFailures",
						Expr:  intstr.FromString(`sum(rate (apiserver_audit_error_total{plugin!="log",job="kube-apiserver"}[5m])) / sum(rate(apiserver_audit_event_total{job="kube-apiserver"}[5m])) > bool 0.02 == 1`),
						For:   ptr.To(monitoringv1.Duration("15m")),
						Labels: map[string]string{
							"service":    "auditlog",
							"severity":   "warning",
							"type":       "seed",
							"visibility": "operator",
						},
						Annotations: map[string]string{
							"summary":     "The kubernetes API server has too many failed attempts to log audit events",
							"description": "The API servers cumulative failure rate in logging audit events is greater than 2%.",
						},
					},
					{
						Record: "shoot:apiserver_audit_event_total:sum",
						Expr:   intstr.FromString(`sum(rate(apiserver_audit_event_total{job="kube-apiserver"}[5m]))`),
					},
					{
						Record: "shoot:apiserver_audit_error_total:sum",
						Expr:   intstr.FromString(`sum(rate(apiserver_audit_error_total{plugin!="log",job="kube-apiserver"}[5m]))`),
					},

					// API latency
					{
						Record: "shoot:apiserver_latency_seconds:quantile",
						Expr:   intstr.FromString(`histogram_quantile(0.99, sum without (instance, pod) (rate(apiserver_request_duration_seconds_bucket[5m])))`),
						Labels: map[string]string{"quantile": "0.99"},
					},
					{
						Record: "shoot:apiserver_latency_seconds:quantile",
						Expr:   intstr.FromString(`histogram_quantile(0.9, sum without (instance, pod) (rate(apiserver_request_duration_seconds_bucket[5m])))`),
						Labels: map[string]string{"quantile": "0.9"},
					},
					{
						Record: "shoot:apiserver_latency_seconds:quantile",
						Expr:   intstr.FromString(`histogram_quantile(0.5, sum without (instance, pod) (rate(apiserver_request_duration_seconds_bucket[5m])))`),
						Labels: map[string]string{"quantile": "0.5"},
					},

					// API server request duration greater than 1s percentage
					{
						Record: "shoot:apiserver_latency:percentage",
						Expr:   intstr.FromString(`1 - sum(rate(apiserver_request_duration_seconds_bucket{le="1",subresource!~"log|portforward|exec|proxy|attach",verb!~"CONNECT|LIST|WATCH",resource="apiservices"}[1h])) / sum(rate(apiserver_request_duration_seconds_count{subresource!~"log|portforward|exec|proxy|attach",verb!~"CONNECT|LIST|WATCH",resource="apiservices"}[1h]))`),
					},

					{
						Record: "shoot:kube_apiserver:sum_by_pod",
						Expr:   intstr.FromString(`sum(up{job="kube-apiserver"}) by (pod)`),
					},

					// API failure rate
					{
						Alert: "ApiserverRequestsFailureRate",
						Expr:  intstr.FromString(`max(sum by(instance,resource,verb) (rate(apiserver_request_total{code=~"5.."}[10m])) / sum by(instance,resource,verb) (rate(apiserver_request_total[10m]))) * 100 > 10`),
						For:   ptr.To(monitoringv1.Duration("30m")),
						Labels: map[string]string{
							"service":    v1beta1constants.DeploymentNameKubeAPIServer,
							"severity":   "warning",
							"type":       "seed",
							"visibility": "operator",
						},
						Annotations: map[string]string{
							"summary":     "Kubernetes API server failure rate is high",
							"description": "The API Server requests failure rate exceeds 10%.",
						},
					},
				},
			}},
		}
		return nil
	})
	return err
}
