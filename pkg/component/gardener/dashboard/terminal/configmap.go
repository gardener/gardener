// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package terminal

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const dataKeyConfig = "config.yaml"

func (t *terminal) configMap() *corev1.ConfigMap {
	// This is the name of ServiceAccounts when the Gardener Dashboard creates Terminal resources, see
	// https://github.com/gardener/dashboard/blob/b99a6ef584eec26dee95028d755f5ebdf5973b2c/backend/lib/services/terminals/index.js#L45-L46
	dashboardServiceAccountName := "dashboard-webterminal"

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: t.namespace,
			Labels:    getLabels(),
		},
		Data: map[string]string{dataKeyConfig: `apiVersion: dashboard.gardener.cloud/v1alpha1
kind: ControllerManagerConfiguration
controllers:
  serviceAccount:
    allowedServiceAccountNames:
    - ` + dashboardServiceAccountName + `
honourCleanupProjectMembership: true
honourServiceAccountRefHostCluster: false
leaderElection:
  leaderElect: true
  resourceNamespace: ` + metav1.NamespaceSystem + `
server:
  healthProbes:
    port: ` + strconv.Itoa(portProbes) + `
  metrics:
    port: ` + strconv.Itoa(portMetrics) + `
`},
	}

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))
	return configMap
}
