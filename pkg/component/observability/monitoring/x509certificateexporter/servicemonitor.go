// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package x509certificateexporter

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/garden"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
)

func (x *x509CertificateExporter) serviceMonitor(resName string, selector labels.Set) *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
		ObjectMeta: monitoringutils.ConfigObjectMeta(resName, x.namespace, garden.Label),
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{MatchLabels: selector},
			Endpoints: []monitoringv1.Endpoint{{
				TargetPort: ptr.To(intstr.FromInt32(port)),
				MetricRelabelConfigs: monitoringutils.StandardMetricRelabelConfig(
					"x509_cert_not_before",
					"x509_cert_not_after",
					"x509_cert_expired",
					"x509_cert_expires_in_seconds",
					"x509_cert_valid_since_seconds",
					"x509_cert_error",
					"x509_read_errors",
				),
			}},
		},
	}
}
