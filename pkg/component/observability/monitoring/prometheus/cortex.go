// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const (
	containerNameCortex   = "cortex"
	cortexConfigNameInfix = "cortex"
	portCortex            = 9091

	dataKeyCortexConfig         = "config.yaml"
	volumeNameCortexConfig      = "cortex-config"
	volumeMountPathCortexConfig = "/etc/cortex/config"

	cortexTarget = "query-frontend"
)

func (p *prometheus) cortexContainer() corev1.Container {
	return corev1.Container{
		Name:            containerNameCortex,
		Image:           p.values.Cortex.Image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Args: []string{
			"-target=" + cortexTarget,
			"-config.file=" + volumeMountPathCortexConfig + "/" + dataKeyCortexConfig,
		},
		Ports: []corev1.ContainerPort{{
			Name:          "frontend",
			ContainerPort: portCortex,
			Protocol:      corev1.ProtocolTCP,
		}},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			ReadOnlyRootFilesystem:   ptr.To(true),
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("300Mi"),
			},
		},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      volumeNameCortexConfig,
			MountPath: volumeMountPathCortexConfig,
			ReadOnly:  true,
		}},
	}
}

func (p *prometheus) cortexVolume(configMapName string) corev1.Volume {
	return corev1.Volume{
		Name:         volumeNameCortexConfig,
		VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: configMapName}}},
	}
}

func (p *prometheus) cortexConfigMap() *corev1.ConfigMap {
	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.name() + "-" + cortexConfigNameInfix,
			Namespace: p.namespace,
			Labels:    p.getLabels(),
		},
		Data: map[string]string{
			dataKeyCortexConfig: `target: ` + cortexTarget + `
auth_enabled: false
http_prefix:
api:
  response_compression_enabled: true
server:
  http_listen_port: ` + strconv.Itoa(portCortex) + `
frontend:
  downstream_url: http://localhost:` + strconv.Itoa(port) + `
  log_queries_longer_than: -1s
query_range:
  split_queries_by_interval: 24h
  align_queries_with_step: true
  cache_results: true
  results_cache:
    cache:
      enable_fifocache: true
      fifocache:
        max_size_bytes: ` + p.values.StorageCapacity.String() + `
        validity: ` + p.values.Cortex.CacheValidity.String() + `
`,
		},
	}

	utilruntime.Must(kubernetesutils.MakeUnique(obj))
	return obj
}
