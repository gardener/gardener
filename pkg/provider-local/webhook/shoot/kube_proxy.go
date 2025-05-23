// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func (m *mutator) mutateKubeProxyConfigMap(_ context.Context, configmap *corev1.ConfigMap) error {
	rawConfig, ok := configmap.Data["config.yaml"]
	if !ok {
		return nil
	}
	config, err := m.kubeProxyConfigCodec.Decode(rawConfig)
	if err != nil {
		return err
	}

	config.Conntrack.MaxPerCore = ptr.To[int32](0)

	data, err := m.kubeProxyConfigCodec.Encode(config)
	if err != nil {
		return err
	}
	configmap.Data["config.yaml"] = data

	return nil
}
