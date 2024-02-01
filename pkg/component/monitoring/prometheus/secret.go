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

package prometheus

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/utils"
)

func (p *prometheus) secretAdditionalScrapeConfigs() *corev1.Secret {
	var scrapeConfigs strings.Builder

	for _, config := range p.values.CentralConfigs.AdditionalScrapeConfigs {
		scrapeConfigs.WriteString(fmt.Sprintf("- %s\n", utils.Indent(config, 2)))
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.name() + "-additional-scrape-configs",
			Namespace: p.namespace,
			Labels:    p.getLabels(),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{dataKeyAdditionalScrapeConfigs: []byte(scrapeConfigs.String())},
	}
}
