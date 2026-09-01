// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	corev1 "k8s.io/api/core/v1"
)

func (m *mutator) mutateKubeProxyConfigMap(configMap *corev1.ConfigMap) error {
	config, err := m.kubeProxyConfigCodec.Decode(configMap.Data[dataKeyConfig])
	if err != nil {
		return err
	}

	config.Conntrack.MaxPerCore = new(int32(0))

	data, err := m.kubeProxyConfigCodec.Encode(config)
	if err != nil {
		return err
	}
	configMap.Data[dataKeyConfig] = data

	return nil
}
