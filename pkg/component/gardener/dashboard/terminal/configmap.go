// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package terminal

import (
	"encoding/json"
	"fmt"

	terminalv1alpha1 "github.com/gardener/terminal-controller-manager/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/ptr"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const dataKeyConfig = "config.yaml"

func (t *terminal) configMap() (*corev1.ConfigMap, error) {
	config := &terminalv1alpha1.ControllerManagerConfiguration{
		APIVersion: "dashboard.gardener.cloud/v1alpha1",
		Kind:       "ControllerManagerConfiguration",
		Server: terminalv1alpha1.ServerConfiguration{
			HealthProbes: &terminalv1alpha1.Server{Port: portProbes},
			Metrics:      &terminalv1alpha1.Server{Port: portMetrics},
		},
		LeaderElection: &componentbaseconfigv1alpha1.LeaderElectionConfiguration{
			LeaderElect:       ptr.To(true),
			ResourceNamespace: metav1.NamespaceSystem,
		},
		HonourServiceAccountRefHostCluster: ptr.To(false),
	}

	rawConfig, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed marshalling config: %w", err)
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: t.namespace,
			Labels:    getLabels(),
		},
		Data: map[string]string{dataKeyConfig: string(rawConfig)},
	}

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))
	return configMap, nil
}
