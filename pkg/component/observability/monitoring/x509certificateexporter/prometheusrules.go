// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0package x509certificateexporter

package x509certificateexporter

import (
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (x *x509CertificateExporter) prometheusRule(labelz labels.Set) *monitoringv1.PrometheusRule {
	var (
		alertDurationCalculation = x.conf.alerting.DurationForAlertEvaluation
		certRenewalExpr          = intstr.FromString(fmt.Sprintf(
			"(x509_cert_not_after - time()) < (%d * 86400)", x.conf.alerting.CertificateRenewalDays,
		))
		certExpirationExpr = intstr.FromString(fmt.Sprintf(
			"(x509_cert_not_after - time()) < (%d * 86400)", x.conf.alerting.CertificateExpirationDays,
		))
		certExpiresTodayExpr = intstr.FromString("(x509_cert_not_after - time()) < 86400")
		genAlertLabels       = func(sev prometheusRuleSeverity) map[string]string {
			return map[string]string{
				"service":          "x509-certificate-exporter",
				"topology":         x.values.PrometheusInstance,
				defaultSeverityKey: string(sev),
			}
		}
		readErrorsLabels       = genAlertLabels(x.conf.alerting.ReadErrorsSeverity)
		certificateErrorLabels = genAlertLabels(x.conf.alerting.CertificateErrorsSeverity)
		renewalLabels          = genAlertLabels(x.conf.alerting.RenewalSeverity)
		expirationLabels       = genAlertLabels(x.conf.alerting.ExpirationSeverity)
		expiresTodayLabels     = genAlertLabels(x.conf.alerting.ExpiresTodaySeverity)
	)

	labelz["prometheus"] = x.values.PrometheusInstance

	return &monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Labels:    labelz,
			Name:      x.values.PrometheusInstance + promRuleName,
			Namespace: x.namespace,
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{
				{
					Name: x.conf.alerting.PrometheusRuleName,
					Rules: []monitoringv1.Rule{
						{
							Alert: "X509ExporterReadErrors",
							Annotations: map[string]string{
								"description": fmt.Sprintf("Over the last %s, this x509-certificate-exporter instance has experienced errors reading certificate files or querying the Kubernetes API. This could be caused by a misconfiguration if triggered when the exporter starts.", x.conf.alerting.DurationForAlertEvaluation),
								"summary":     "Increasing read errors for x509-certificate-exporter on {{$externalLabels.landscape}} landscape",
							},
							Expr:   intstr.FromString(fmt.Sprintf("increase(x509_read_errors[%s]) > 0", x.conf.alerting.DurationForAlertEvaluation)),
							Labels: readErrorsLabels,
						},
						{
							Alert: "CertificateError",
							Annotations: map[string]string{
								"description": `Certificate could not be decoded {{ if $labels.secret_name -}} in Kubernetes secret "{{ $labels.secret_namespace }}/{{ $labels.secret_name }}" {{- else -}} at location "{{ $labels.filepath }}" {{- end }}`,
								"summary":     "Certificate cannot be decoded on {{ $externalLabels.landscape }} landscape",
							},
							Expr:   intstr.FromString("x509_cert_error > 0"),
							For:    &alertDurationCalculation,
							Labels: certificateErrorLabels,
						},
						{
							Alert: "CertificateRenewal",
							Annotations: map[string]string{
								"description": `Certificate for "{{ $labels.subject_CN }}" should be renewed {{ if $labels.secret_name -}} in Kubernetes secret "{{ $labels.secret_namespace }}/{{ $labels.secret_name }}" {{- else -}} at location "{{ $labels.filepath }}" {{- end }}`,
								"summary":     "Certificate should be renewed on {{ $externalLabels.landscape }} landscape",
							},
							Expr:   certRenewalExpr,
							Labels: renewalLabels,
						},
						{
							Alert: "CertificateExpiration",
							Annotations: map[string]string{
								"description": `Certificate for "{{ $labels.subject_CN }}" is about to expire after {{ humanizeDuration $value }} {{ if $labels.secret_name -}} in Kubernetes secret "{{ $labels.secret_namespace }}/{{ $labels.secret_name }}" {{- else -}} at location "{{ $labels.filepath }}" {{- end }}`,
								"summary":     "Certificate is about to expire on {{ $externalLabels.landscape }} landscape",
							},
							Expr:   certExpirationExpr,
							Labels: expirationLabels,
						},
						{
							Alert: "CertificateExpiresToday",
							Annotations: map[string]string{
								"description": `Certificate for "{{ $labels.subject_CN }}" is about to expire TODAY {{- if $labels.secret_name -}} in Kubernetes secret "{{ $labels.secret_namespace }}/{{ $labels.secret_name }}" {{- else -}} at location "{{ $labels.filepath }}"{{- end }}`,
								"summary":     "Certificate expires today on {{ $externalLabels.landscape }} landscape",
							},
							Expr:   certExpiresTodayExpr,
							Labels: expiresTodayLabels,
						},
					},
				},
			},
		},
	}
}
