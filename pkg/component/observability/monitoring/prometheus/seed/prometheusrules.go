// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	_ "embed"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
)

var (
	//go:embed assets/prometheusrules/healthcheck.yaml
	healthcheckYAML []byte
	healthcheck     *monitoringv1.PrometheusRule
)

func init() {
	healthcheck = &monitoringv1.PrometheusRule{}
	utilruntime.Must(runtime.DecodeInto(monitoringutils.Decoder, healthcheckYAML, healthcheck))
}

// CentralPrometheusRules returns the central PrometheusRule resources for the seed prometheus.
func CentralPrometheusRules() []*monitoringv1.PrometheusRule {
	return []*monitoringv1.PrometheusRule{
		healthcheck.DeepCopy(),
	}
}
