// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package longterm

import (
	_ "embed"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
)

var (
	//go:embed assets/prometheusrules/recording.yaml
	recordingYAML []byte
	recording     *monitoringv1.PrometheusRule

	//go:embed assets/prometheusrules/sla.yaml
	slaYAML []byte
	sla     *monitoringv1.PrometheusRule
)

func init() {
	recording = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, recordingYAML, recording))

	sla = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, slaYAML, sla))
}

// CentralPrometheusRules returns the central PrometheusRule resources for the long-term prometheus.
func CentralPrometheusRules() []*monitoringv1.PrometheusRule {
	return []*monitoringv1.PrometheusRule{
		recording.DeepCopy(),
		sla.DeepCopy(),
	}
}
