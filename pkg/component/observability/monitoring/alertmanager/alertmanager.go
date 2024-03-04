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

package alertmanager

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils"
)

func (a *alertManager) alertManager(takeOverOldPV bool) *monitoringv1.Alertmanager {
	obj := &monitoringv1.Alertmanager{
		ObjectMeta: metav1.ObjectMeta{
			Name:      a.values.Name,
			Namespace: a.namespace,
			Labels:    a.getLabels(),
		},
		Spec: monitoringv1.AlertmanagerSpec{
			PodMetadata: &monitoringv1.EmbeddedObjectMetadata{
				Labels: utils.MergeStringMaps(a.getLabels(), map[string]string{
					v1beta1constants.LabelNetworkPolicyToDNS:             v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyToPublicNetworks:  v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyToPrivateNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
				}),
			},
			PriorityClassName: a.values.PriorityClassName,
			Replicas:          &a.values.Replicas,
			Image:             &a.values.Image,
			ImagePullPolicy:   corev1.PullIfNotPresent,
			Version:           a.values.Version,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("5m"),
					corev1.ResourceMemory: resource.MustParse("20Mi"),
				},
			},
			SecurityContext: &corev1.PodSecurityContext{RunAsUser: ptr.To(int64(0))},
			Storage: &monitoringv1.StorageSpec{
				VolumeClaimTemplate: monitoringv1.EmbeddedPersistentVolumeClaim{
					EmbeddedObjectMetadata: monitoringv1.EmbeddedObjectMetadata{Name: "alertmanager-db"},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: a.values.StorageCapacity}},
					},
				},
			},
			AlertmanagerConfigSelector:          &metav1.LabelSelector{MatchLabels: map[string]string{"alertmanager": a.values.Name}},
			AlertmanagerConfigNamespaceSelector: &metav1.LabelSelector{},
			LogLevel:                            "info",
			ForceEnableClusterMode:              true,
		},
	}

	if a.hasSMTPSecret() {
		obj.Spec.AlertmanagerConfiguration = &monitoringv1.AlertmanagerConfiguration{Name: a.name()}
	}

	if a.values.Replicas > 1 {
		obj.Spec.PodMetadata.Labels["networking.resources.gardener.cloud/to-alertmanager-operated-tcp-9094"] = v1beta1constants.LabelNetworkPolicyAllowed
		obj.Spec.PodMetadata.Labels["networking.resources.gardener.cloud/to-alertmanager-operated-udp-9094"] = v1beta1constants.LabelNetworkPolicyAllowed
	}

	if a.values.ClusterType == component.ClusterTypeShoot {
		obj.Labels[v1beta1constants.GardenRole] = v1beta1constants.GardenRoleMonitoring
		obj.Spec.PodMetadata.Labels[v1beta1constants.GardenRole] = v1beta1constants.GardenRoleMonitoring
	}

	if a.values.Ingress != nil {
		obj.Spec.ExternalURL = "https://" + a.values.Ingress.Host
	}

	if takeOverOldPV {
		obj.Spec.InitContainers = append(obj.Spec.InitContainers, corev1.Container{
			Name:            "take-over-old-pv",
			Image:           a.values.DataMigration.ImageAlpine,
			ImagePullPolicy: corev1.PullIfNotPresent,
			VolumeMounts:    []corev1.VolumeMount{{Name: "alertmanager-db", MountPath: "/alertmanager"}},
			Command:         []string{"/bin/sh", "-c"},
			Args:            []string{`if [[ -d /alertmanager/alertmanager- ]]; then mv /alertmanager/alertmanager- /alertmanager/alertmanager-db; else echo "rename already done"; fi`},
		})
	}

	return obj
}
