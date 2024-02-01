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
func StandardMetricRelabelConfig(allowedMetrics ...string) []*monitoringv1.RelabelConfig {
	return []*monitoringv1.RelabelConfig{{
		SourceLabels: []monitoringv1.LabelName{"__name__"},
		Action:       "keep",
		Regex:        `^(` + strings.Join(allowedMetrics, "|") + `)$`,
	}}
}

// Labels returns the labels for the respective prometheus instance.
func Labels(prometheusName string) map[string]string {
	return map[string]string{"prometheus": prometheusName}
}
