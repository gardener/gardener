// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package aggregate

import (
	_ "embed"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
)

var (
	//go:embed assets/prometheusrules/metering.rules.stateful.yaml
	meteringStatefulYAML []byte
	meteringStateful     *monitoringv1.PrometheusRule
)

func init() {
	meteringStateful = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, meteringStatefulYAML, meteringStateful))
}

// CentralPrometheusRules returns the central PrometheusRule resources for the aggregate prometheus.
func CentralPrometheusRules(seedIsGarden bool) []*monitoringv1.PrometheusRule {
	rules := []monitoringv1.Rule{
		{
			Alert: "PodStuckInPending",
			Expr:  intstr.FromString(`sum_over_time(kube_pod_status_phase{phase="Pending"}[5m]) > 0`),
			For:   ptr.To(monitoringv1.Duration("10m")),
			Labels: map[string]string{
				"severity":   "warning",
				"type":       "seed",
				"visibility": "operator",
			},
			Annotations: map[string]string{
				"description": "Pod {{$labels.pod}} in namespace {{$labels.namespace}} was stuck in Pending state for more than 10 minutes.",
				"summary":     "A pod is stuck in pending",
			},
		},
		{
			Alert: "NodeNotHealthy",
			Expr:  intstr.FromString(`count_over_time((sum by (node) (kube_node_spec_taint{effect="NoSchedule", key!~"node.kubernetes.io/unschedulable|deployment.machine.sapcloud.io/prefer-no-schedule|node-role.kubernetes.io/control-plane|ToBeDeletedByClusterAutoscaler|` + v1beta1constants.TaintNodeCriticalComponentsNotReady + `"}))[30m:]) > 9`),
			For:   ptr.To(monitoringv1.Duration("0m")),
			Labels: map[string]string{
				"severity":   "warning",
				"type":       "seed",
				"visibility": "operator",
			},
			Annotations: map[string]string{
				"description": "Node {{$labels.node}} in seed {{$externalLabels.seed}} was not healthy for ten scrapes in the past 30 mins.",
				"summary":     "A node is not healthy.",
			},
		},
		{
			Alert: "TooManyEtcdSnapshotCompactionJobsFailing",
			Expr:  intstr.FromString(`count(increase(etcddruid_compaction_jobs_total{succeeded="false"}[3h]) >= 1) / count(increase(etcddruid_compaction_jobs_total[3h])) > 0.1`),
			Labels: map[string]string{
				"severity":   "warning",
				"type":       "seed",
				"visibility": "operator",
			},
			Annotations: map[string]string{
				"description": "Seed {{$externalLabels.seed}} has too many etcd snapshot compaction jobs failing in the past 3 hours.",
				"summary":     "Too many etcd snapshot compaction jobs are failing in the seed.",
			},
		},
	}

	// Avoid duplicating the alert when the seed is garden because the garden cluster always deploys the VPA capped recommendation alert.
	if !seedIsGarden {
		rules = append(rules, monitoringv1.Rule{
			Alert: "VerticalPodAutoscalerCappedRecommendation",
			Expr: intstr.FromString(`
  count_over_time(
    (
        {__name__=~"kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_uncappedtarget_.+"}
      >
        {__name__=~"kube_customresource_verticalpodautoscaler_status_recommendation_containerrecommendations_target_.+"}
    )[5m:]
  )
==
  5`),
			Labels: map[string]string{
				"severity":   "warning",
				"type":       "seed",
				"visibility": "operator",
			},
			Annotations: map[string]string{
				"summary": "A VPA recommendation in a seed is capped.",
				"description": "The following VPA from a seed shows a " +
					"{{ if eq .Labels.unit \"core\" -}} CPU {{- else if eq .Labels.unit \"byte\" -}} memory {{- end }} " +
					"uncapped target recommendation larger than the regular target recommendation:\n" +
					"- seed = {{ $externalLabels.seed }}\n" +
					"- namespace = {{ $labels.namespace }}\n" +
					"- vpa = {{ $labels.verticalpodautoscaler }}\n" +
					"- container = {{ $labels.container }}",
			},
		})
	}

	return []*monitoringv1.PrometheusRule{
		meteringStateful.DeepCopy(),
		{
			ObjectMeta: metav1.ObjectMeta{Name: "seed"},
			Spec: monitoringv1.PrometheusRuleSpec{
				Groups: []monitoringv1.RuleGroup{{
					Name:  "seed.rules",
					Rules: rules,
				}},
			},
		},
	}
}
