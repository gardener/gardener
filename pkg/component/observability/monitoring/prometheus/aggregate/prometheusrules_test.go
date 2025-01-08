// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package aggregate_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/aggregate"
)

var _ = Describe("PrometheusRules", func() {
	Describe("#CentralPrometheusRules", func() {
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
				Expr:  intstr.FromString(`count_over_time((sum by (node) (kube_node_spec_taint{effect="NoSchedule", key!~"node.kubernetes.io/unschedulable|deployment.machine.sapcloud.io/prefer-no-schedule|ToBeDeletedByClusterAutoscaler|` + v1beta1constants.TaintNodeCriticalComponentsNotReady + `"}))[30m:]) > 9`),
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
		It("should return the expected objects", func() {
			Expect(aggregate.CentralPrometheusRules()).To(HaveExactElements(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"TypeMeta":   MatchFields(IgnoreExtras, Fields{"APIVersion": Equal("monitoring.coreos.com/v1"), "Kind": Equal("PrometheusRule")}),
					"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("metering-stateful")}),
				})),
				Equal(&monitoringv1.PrometheusRule{
					ObjectMeta: metav1.ObjectMeta{Name: "seed"},
					Spec: monitoringv1.PrometheusRuleSpec{
						Groups: []monitoringv1.RuleGroup{{
							Name:  "seed.rules",
							Rules: rules,
						}},
					},
				}),
			))
		})
	})
})
