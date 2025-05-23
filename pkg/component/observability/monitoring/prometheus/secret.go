// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/utils"
)

const (
	secretNameSuffixAdditionalScrapeConfigs       = "-additional-scrape-configs"
	secretNameSuffixAdditionalAlertRelabelConfigs = "-additional-alert-relabel-configs"
	secretNameSuffixAdditionalAlertmanagerConfigs = "-additional-alertmanager-configs"
	secretNameSuffixRemoteWriteBasicAuth          = "-remote-write-basic-auth"
)

func (p *prometheus) secretAdditionalScrapeConfigs() *corev1.Secret {
	var scrapeConfigs strings.Builder

	for _, config := range p.values.CentralConfigs.AdditionalScrapeConfigs {
		scrapeConfigs.WriteString(fmt.Sprintf("- %s\n", utils.Indent(config, 2)))
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.name() + secretNameSuffixAdditionalScrapeConfigs,
			Namespace: p.namespace,
			Labels:    p.getLabels(),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{dataKeyAdditionalScrapeConfigs: []byte(scrapeConfigs.String())},
	}
}

func (p *prometheus) secretAdditionalAlertmanagerConfigs() *corev1.Secret {
	if p.values.Alerting == nil || p.values.Alerting.AdditionalAlertmanager == nil {
		return nil
	}

	config := `
static_configs:
- targets:
  - ` + string(p.values.Alerting.AdditionalAlertmanager["url"])

	switch string(p.values.Alerting.AdditionalAlertmanager["auth_type"]) {
	case "basic":
		config += `
basic_auth:
  username: ` + string(p.values.Alerting.AdditionalAlertmanager["username"]) + `
  password: ` + string(p.values.Alerting.AdditionalAlertmanager["password"])

	case "certificate":
		config += `
tls_config:
  ca: ` + string(p.values.Alerting.AdditionalAlertmanager["ca.crt"]) + `
  cert: ` + string(p.values.Alerting.AdditionalAlertmanager["tls.crt"]) + `
  key: ` + string(p.values.Alerting.AdditionalAlertmanager["tls.key"]) + `
  insecure_skip_verify: ` + string(p.values.Alerting.AdditionalAlertmanager["insecure_skip_verify"])
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.name() + secretNameSuffixAdditionalAlertmanagerConfigs,
			Namespace: p.namespace,
			Labels:    p.getLabels(),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{dataKeyAdditionalAlertmanagerConfigs: []byte(config)},
	}
}

func (p *prometheus) secretRemoteWriteBasicAuth() *corev1.Secret {
	if p.values.RemoteWrite == nil || p.values.RemoteWrite.GlobalShootRemoteWriteSecret == nil {
		return nil
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.name() + secretNameSuffixRemoteWriteBasicAuth,
			Namespace: p.namespace,
			Labels:    p.getLabels(),
		},
		Type: corev1.SecretTypeOpaque,
		Data: p.values.RemoteWrite.GlobalShootRemoteWriteSecret.Data,
	}
}
