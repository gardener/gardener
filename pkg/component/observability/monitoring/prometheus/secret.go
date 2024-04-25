// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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

func (p *prometheus) secretAdditionalAlertRelabelConfigs() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.name() + secretNameSuffixAdditionalAlertRelabelConfigs,
			Namespace: p.namespace,
			Labels:    p.getLabels(),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{dataKeyAdditionalAlertRelabelConfigs: []byte(`
- source_labels: [ ignoreAlerts ]
  regex: true
  action: drop
`)},
	}
}
