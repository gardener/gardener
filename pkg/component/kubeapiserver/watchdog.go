// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

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
