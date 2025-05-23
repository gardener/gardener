// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package alertmanager

import (
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/component"
)

const dataKeyAuthPassword = "auth_password"

func (a *alertManager) config() *monitoringv1alpha1.AlertmanagerConfig {
	if !a.hasSMTPSecret() {
		return nil
	}

	var (
		emailReceiverName = "email-kubernetes-ops"
		visibility        = "operator"
	)

	if a.values.ClusterType == component.ClusterTypeShoot {
		visibility = "owner"
	}

	return &monitoringv1alpha1.AlertmanagerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      a.name(),
			Namespace: a.namespace,
		},
		Spec: monitoringv1alpha1.AlertmanagerConfigSpec{
			// The root route on which each incoming alert enters.
			Route: &monitoringv1alpha1.Route{
				// The labels by which incoming alerts are grouped together.
				GroupBy: []string{"service"},
				// When a new group of alerts is created by an incoming alert, wait at least 'group_wait' to send the
				// initial notification.
				// This way ensures that you get multiple alerts for the same group that start firing shortly after
				// another are batched together on the first notification.
				GroupWait: "5m",
				// When the first notification was sent, wait 'group_interval' to send a batch of new alerts that
				// started firing for that group.
				GroupInterval: "5m",
				// If an alert has successfully been sent, wait 'repeat_interval' to resend them.
				RepeatInterval: "72h",
				// Send alerts by default to nowhere
				Receiver: "dev-null",
				// email only for critical and blocker
				Routes: []apiextensionsv1.JSON{{Raw: []byte(`
				  {"matchers": [{"name": "visibility",
				                 "matchType": "=~",
				                 "value": "all|` + visibility + `"}],
				   "receiver": "` + emailReceiverName + `"}`)}},
			},
			InhibitRules: []monitoringv1alpha1.InhibitRule{
				// Apply inhibition if the alert name is the same.
				{
					SourceMatch: []monitoringv1alpha1.Matcher{{Name: "severity", Value: "critical", MatchType: monitoringv1alpha1.MatchEqual}},
					TargetMatch: []monitoringv1alpha1.Matcher{{Name: "severity", Value: "warning", MatchType: monitoringv1alpha1.MatchEqual}},
					Equal:       []string{"alertname", "service", "cluster"},
				},
				// Stop all alerts for type=shoot if there are VPN problems.
				{
					SourceMatch: []monitoringv1alpha1.Matcher{{Name: "service", Value: "vpn", MatchType: monitoringv1alpha1.MatchEqual}},
					TargetMatch: []monitoringv1alpha1.Matcher{{Name: "type", Value: "shoot", MatchType: monitoringv1alpha1.MatchRegexp}},
					Equal:       []string{"type", "cluster"},
				},
				// Stop warning and critical alerts if there is a blocker
				{
					SourceMatch: []monitoringv1alpha1.Matcher{{Name: "severity", Value: "blocker", MatchType: monitoringv1alpha1.MatchEqual}},
					TargetMatch: []monitoringv1alpha1.Matcher{{Name: "severity", Value: "^(critical|warning)$", MatchType: monitoringv1alpha1.MatchRegexp}},
					Equal:       []string{"cluster"},
				},
				// If the API server is down inhibit no worker nodes alert. No worker nodes depends on
				// kube-state-metrics which depends on the API server.
				{
					SourceMatch: []monitoringv1alpha1.Matcher{{Name: "service", Value: "kube-apiserver", MatchType: monitoringv1alpha1.MatchEqual}},
					TargetMatch: []monitoringv1alpha1.Matcher{{Name: "service", Value: "nodes", MatchType: monitoringv1alpha1.MatchRegexp}},
					Equal:       []string{"cluster"},
				},
				// If API server is down inhibit kube-state-metrics alerts.
				{
					SourceMatch: []monitoringv1alpha1.Matcher{{Name: "service", Value: "kube-apiserver", MatchType: monitoringv1alpha1.MatchEqual}},
					TargetMatch: []monitoringv1alpha1.Matcher{{Name: "severity", Value: "info", MatchType: monitoringv1alpha1.MatchRegexp}},
					Equal:       []string{"cluster"},
				},
				// No Worker nodes depends on kube-state-metrics. Inhibit no worker nodes if kube-state-metrics is down.
				{
					SourceMatch: []monitoringv1alpha1.Matcher{{Name: "service", Value: "kube-state-metrics-shoot", MatchType: monitoringv1alpha1.MatchEqual}},
					TargetMatch: []monitoringv1alpha1.Matcher{{Name: "service", Value: "nodes", MatchType: monitoringv1alpha1.MatchRegexp}},
					Equal:       []string{"cluster"},
				},
			},
			Receivers: []monitoringv1alpha1.Receiver{
				{Name: "dev-null"},
				{
					Name:         emailReceiverName,
					EmailConfigs: a.emailConfigs(),
				},
			},
		},
	}
}

func (a *alertManager) smtpSecret() *corev1.Secret {
	if !a.hasSMTPSecret() {
		return nil
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      a.name() + "-smtp",
			Namespace: a.namespace,
		},
		Type: a.values.AlertingSMTPSecret.Type,
		Data: map[string][]byte{dataKeyAuthPassword: a.values.AlertingSMTPSecret.Data[dataKeyAuthPassword]},
	}
}

func (a *alertManager) hasSMTPSecret() bool {
	return a.values.AlertingSMTPSecret != nil && string(a.values.AlertingSMTPSecret.Data["auth_type"]) == "smtp"
}

func (a *alertManager) emailConfigs() []monitoringv1alpha1.EmailConfig {
	emailReceivers := []string{string(a.values.AlertingSMTPSecret.Data["to"])}
	if len(a.values.EmailReceivers) > 0 {
		emailReceivers = a.values.EmailReceivers
	}

	var configs []monitoringv1alpha1.EmailConfig
	for _, email := range emailReceivers {
		configs = append(configs, monitoringv1alpha1.EmailConfig{
			To:           email,
			From:         string(a.values.AlertingSMTPSecret.Data["from"]),
			Smarthost:    string(a.values.AlertingSMTPSecret.Data["smarthost"]),
			AuthUsername: string(a.values.AlertingSMTPSecret.Data["auth_username"]),
			AuthIdentity: string(a.values.AlertingSMTPSecret.Data["auth_identity"]),
			AuthPassword: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: a.smtpSecret().Name},
				Key:                  dataKeyAuthPassword,
			},
		})
	}

	return configs
}
