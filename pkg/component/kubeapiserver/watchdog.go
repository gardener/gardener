// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	watchdogConfigMapNamePrefix = "kube-apiserver-watchdog"
	dataKeyWatchdogScript       = "watchdog.sh"
)

//go:embed resources/watchdog.sh
var watchdogScript string

func (k *kubeAPIServer) reconcileTerminationHandlerConfigMap(ctx context.Context, configMap *corev1.ConfigMap) error {
	configMap.Labels = getLabels()
	configMap.Data = map[string]string{
		dataKeyWatchdogScript: watchdogScript,
	}
	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

	return client.IgnoreAlreadyExists(k.client.Client().Create(ctx, configMap))
}
