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
func CentralPrometheusRules() []*monitoringv1.PrometheusRule {
	return []*monitoringv1.PrometheusRule{
		meteringStateful,
		{
			ObjectMeta: metav1.ObjectMeta{Name: "seed"},
			Spec: monitoringv1.PrometheusRuleSpec{
				Groups: []monitoringv1.RuleGroup{{
					Name: "seed.rules",
					Rules: []monitoringv1.Rule{
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
							Expr:  intstr.FromString(`count_over_time((sum by (node) (kube_node_spec_taint{effect="NoSchedule", key!="deployment.machine.sapcloud.io/prefer-no-schedule", key!="ToBeDeletedByClusterAutoscaler", key!="` + v1beta1constants.LabelNodeCriticalComponent + `"}))[30m:]) > 9`),
							For:   ptr.To(monitoringv1.Duration("0m")),
							Labels: map[string]string{
								"severity":   "warning",
								"type":       "seed",
								"visibility": "operator",
							},
							Annotations: map[string]string{
								"description": "Node {{$labels.node}} in landscape {{$externalLabels.landscape}} was not healthy for ten scrapes in the past 30 mins.",
								"summary":     "A node is not healthy.",
							},
						},
					},
				}},
			},
		},
	}
}
