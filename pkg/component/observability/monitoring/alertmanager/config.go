// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package alertmanager

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener/pkg/component"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
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
			Route: &monitoringv1alpha1.Route{
				GroupBy:        []string{"service"},
				GroupWait:      new(monitoringv1.NonEmptyDuration("5m")),
				GroupInterval:  new(monitoringv1.NonEmptyDuration("5m")),
				RepeatInterval: new(monitoringv1.NonEmptyDuration("72h")),
				Receiver:       "dev-null",
				Routes: []apiextensionsv1.JSON{{Raw: []byte(`
				  {"matchers": [{"name": "visibility",
				                 "matchType": "=~",
				                 "value": "all|` + visibility + `"}],
				   "receiver": "` + emailReceiverName + `"}`)}},
			},
			InhibitRules: []monitoringv1alpha1.InhibitRule{
				{
					SourceMatch: []monitoringv1alpha1.Matcher{{Name: "severity", Value: "critical", MatchType: monitoringv1alpha1.MatchEqual}},
					TargetMatch: []monitoringv1alpha1.Matcher{{Name: "severity", Value: "warning", MatchType: monitoringv1alpha1.MatchEqual}},
					Equal:       []string{"alertname", "service", "cluster"},
				},
				{
					SourceMatch: []monitoringv1alpha1.Matcher{{Name: "service", Value: "vpn", MatchType: monitoringv1alpha1.MatchEqual}},
					TargetMatch: []monitoringv1alpha1.Matcher{{Name: "type", Value: "shoot", MatchType: monitoringv1alpha1.MatchRegexp}},
					Equal:       []string{"type", "cluster"},
				},
				{
					SourceMatch: []monitoringv1alpha1.Matcher{{Name: "severity", Value: "blocker", MatchType: monitoringv1alpha1.MatchEqual}},
					TargetMatch: []monitoringv1alpha1.Matcher{{Name: "severity", Value: "^(critical|warning)$", MatchType: monitoringv1alpha1.MatchRegexp}},
					Equal:       []string{"cluster"},
				},
				{
					SourceMatch: []monitoringv1alpha1.Matcher{{Name: "service", Value: "kube-apiserver", MatchType: monitoringv1alpha1.MatchEqual}},
					TargetMatch: []monitoringv1alpha1.Matcher{{Name: "service", Value: "nodes", MatchType: monitoringv1alpha1.MatchRegexp}},
					Equal:       []string{"cluster"},
				},
				{
					SourceMatch: []monitoringv1alpha1.Matcher{{Name: "service", Value: "kube-apiserver", MatchType: monitoringv1alpha1.MatchEqual}},
					TargetMatch: []monitoringv1alpha1.Matcher{{Name: "severity", Value: "info", MatchType: monitoringv1alpha1.MatchRegexp}},
					Equal:       []string{"cluster"},
				},
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

// customConfig decodes the owner-supplied AlertmanagerConfig from values.AdditionalAlertmanagerConfig and
// returns it ready to be added to the managed resource. It injects the "alertmanager: <name>" label because
// the Alertmanager CR's alertmanagerConfigSelector matches on that label — without it prometheus-operator
// would ignore the config. The resource name is suffixed with "-custom" to avoid collision with the
// Gardener-managed config produced by config().
func (a *alertManager) customConfig() *monitoringv1alpha1.AlertmanagerConfig {
	if len(a.values.AdditionalAlertmanagerConfig) == 0 {
		return nil
	}

	obj := &monitoringv1alpha1.AlertmanagerConfig{}
	if err := runtime.DecodeInto(monitoringutils.Decoder, a.values.AdditionalAlertmanagerConfig, obj); err != nil {
		return nil
	}

	obj.Name = a.name() + "-custom"
	obj.Namespace = a.namespace
	if obj.Labels == nil {
		obj.Labels = map[string]string{}
	}
	obj.Labels["alertmanager"] = a.values.Name
	return obj
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
			To:           new(email),
			From:         new(string(a.values.AlertingSMTPSecret.Data["from"])),
			Smarthost:    new(string(a.values.AlertingSMTPSecret.Data["smarthost"])),
			AuthUsername: new(string(a.values.AlertingSMTPSecret.Data["auth_username"])),
			AuthIdentity: new(string(a.values.AlertingSMTPSecret.Data["auth_identity"])),
			AuthPassword: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: a.smtpSecret().Name},
				Key:                  dataKeyAuthPassword,
			},
		})
	}

	return configs
}
