// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	_ "embed"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
)

var (
	//go:embed assets/prometheusrules/metering.rules.yaml
	meteringYAML []byte
	metering     *monitoringv1.PrometheusRule

	//go:embed assets/prometheusrules/metering.rules.stateful.yaml
	meteringStatefulYAML []byte
	meteringStateful     *monitoringv1.PrometheusRule

	//go:embed assets/prometheusrules/recording-rules.rules.yaml
	recordingRulesYAML []byte
	recordingRules     *monitoringv1.PrometheusRule
)

func init() {
	metering = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, meteringYAML, metering))

	meteringStateful = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, meteringStatefulYAML, meteringStateful))

	recordingRules = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, recordingRulesYAML, recordingRules))
}

// CentralPrometheusRules returns the central PrometheusRule resources for the cache prometheus.
func CentralPrometheusRules() []*monitoringv1.PrometheusRule {
	return []*monitoringv1.PrometheusRule{metering.DeepCopy(), meteringStateful.DeepCopy(), recordingRules.DeepCopy()}
}
