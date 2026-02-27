// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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

	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
)

var (
	//go:embed assets/prometheusrules/healthcheck.yaml
	healthcheckYAML []byte
	healthcheck     *monitoringv1.PrometheusRule

	//go:embed assets/prometheusrules/metering.rules.stateful.yaml
	meteringStatefulYAML []byte
	meteringStateful     *monitoringv1.PrometheusRule

	//go:embed assets/prometheusrules/pvc.yaml
	pvcYAML []byte
	pvc     *monitoringv1.PrometheusRule
)

func init() {
	healthcheck = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, healthcheckYAML, healthcheck))

	meteringStateful = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, meteringStatefulYAML, meteringStateful))

	pvc = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, pvcYAML, pvc))
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
			Alert: "EtcdSnapshotCompactionJobsFailingForSeed",
			Expr:  intstr.FromString(`count(count by (etcd_namespace) (increase(etcddruid_compaction_jobs_total{succeeded="false", failureReason=~"processFailure|unknown"}[3h]) >= 1)) / count(count by (etcd_namespace) (increase(etcddruid_compaction_jobs_total[3h]))) > 0.1`),
			Labels: map[string]string{
				"severity":   "warning",
				"type":       "seed",
				"visibility": "operator",
			},
			Annotations: map[string]string{
				"description": "Seed {{$externalLabels.seed}} has more than 10 percent of shoot namespaces with at least one etcd snapshot compaction job failure in the past 3 hours.",
				"summary":     "Too many shoot namespaces have failing etcd snapshot compaction jobs.",
			},
		},
		{
			Alert: "EtcdSnapshotCompactionJobsFailingForNamespace",
			Expr:  intstr.FromString(`sum by (etcd_namespace) (increase(etcddruid_compaction_jobs_total{succeeded="false", failureReason=~"processFailure|unknown"}[3h])) / sum by (etcd_namespace) (increase(etcddruid_compaction_jobs_total[3h])) > 0.1`),
			Labels: map[string]string{
				"severity":   "warning",
				"type":       "seed",
				"visibility": "operator",
			},
			Annotations: map[string]string{
				"description": "Namespace {{$labels.etcd_namespace}} on seed {{$externalLabels.seed}} has more than 10 percent of etcd snapshot compaction jobs failing in the past 3 hours.",
				"summary":     "Too many etcd snapshot compaction jobs are failing for a specific namespace.",
			},
		},
		{
			Alert: "EtcdFullSnapshotsFailingForNamespace",
			Expr:  intstr.FromString(`sum by (etcd_namespace) (increase(etcddruid_compaction_full_snapshot_triggered_total{succeeded="false"}[3h])) / sum by (etcd_namespace) (increase(etcddruid_compaction_full_snapshot_triggered_total[3h])) > 0.1`),
			Labels: map[string]string{
				"severity":   "warning",
				"type":       "seed",
				"visibility": "operator",
			},
			Annotations: map[string]string{
				"description": "Namespace {{$labels.etcd_namespace}} on seed {{$externalLabels.seed}} has more than 10 percent of etcd full snapshots (triggered by compaction controller) failing in the past 3 hours.",
				"summary":     "Too many etcd full snapshots are failing for a specific namespace.",
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
		healthcheck.DeepCopy(),
		meteringStateful.DeepCopy(),
		pvc.DeepCopy(),
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
