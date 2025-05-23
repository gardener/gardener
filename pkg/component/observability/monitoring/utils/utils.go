// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"strings"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigObjectMeta returns the object meta for a standard Prometheus config object (e.g., ServiceMonitor).
func ConfigObjectMeta(name, namespace, prometheusName string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      prometheusName + "-" + name,
		Namespace: namespace,
		Labels:    Labels(prometheusName),
	}
}

// StandardMetricRelabelConfig returns the standard relabel config for metrics.
func StandardMetricRelabelConfig(allowedMetrics ...string) []monitoringv1.RelabelConfig {
	return []monitoringv1.RelabelConfig{{
		SourceLabels: []monitoringv1.LabelName{"__name__"},
		Action:       "keep",
		Regex:        `^(` + strings.Join(allowedMetrics, "|") + `)$`,
	}}
}

// Labels returns the labels for the respective prometheus instance.
func Labels(prometheusName string) map[string]string {
	return map[string]string{"prometheus": prometheusName}
}
