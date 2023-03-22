// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
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

	config.Conntrack.MaxPerCore = pointer.Int32(0)

	data, err := m.kubeProxyConfigCodec.Encode(config)
	if err != nil {
		return err
	}
	configmap.Data["config.yaml"] = data

	return nil
}
