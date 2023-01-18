// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubeapiserver

import (
	"context"
	_ "embed"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/controllerutils"
)

const (
	watchdogConfigMapName = "kube-apiserver-watchdog"
	dataKeyWatchdogScript = "watchdog.sh"
)

//go:embed resources/watchdog.sh
var watchdogScript string

func (k *kubeAPIServer) reconcileTerminationHandlerConfigMap(ctx context.Context) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      watchdogConfigMapName,
			Namespace: k.namespace,
			Labels:    getLabels(),
		},
	}
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, k.client.Client(), configMap, func() error {
		configMap.Data = map[string]string{
			dataKeyWatchdogScript: watchdogScript,
		}

		return nil
	})

	return err
}
