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

package prometheus

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	monitoringutils "github.com/gardener/gardener/pkg/component/monitoring/utils"
)

func (p *prometheus) prometheus(takeOverOldPV bool) *monitoringv1.Prometheus {
	reloadStrategy := monitoringv1.HTTPReloadStrategyType

	obj := &monitoringv1.Prometheus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.values.Name,
			Namespace: p.namespace,
			Labels:    p.getLabels(),
		},
		Spec: monitoringv1.PrometheusSpec{
			Retention:          "1d",
			RetentionSize:      "5GB",
			EvaluationInterval: "1m",
			CommonPrometheusFields: monitoringv1.CommonPrometheusFields{
				ScrapeInterval: "1m",
				ReloadStrategy: &reloadStrategy,
				AdditionalScrapeConfigs: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: p.name() + "-additional-scrape-configs",
					},
					Key: dataKeyAdditionalScrapeConfigs,
				},

				PodMetadata: &monitoringv1.EmbeddedObjectMetadata{
					Labels: map[string]string{
						v1beta1constants.LabelNetworkPolicyToDNS:                                                         v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer:                                            v1beta1constants.LabelNetworkPolicyAllowed,
						"networking.resources.gardener.cloud/to-" + v1beta1constants.LabelNetworkPolicySeedScrapeTargets: v1beta1constants.LabelNetworkPolicyAllowed,
					},
				},
				PriorityClassName: p.values.PriorityClassName,
				Replicas:          ptr.To(int32(1)),
				Image:             &p.values.Image,
				ImagePullPolicy:   corev1.PullIfNotPresent,
				Version:           p.values.Version,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("300m"),
						corev1.ResourceMemory: resource.MustParse("1000Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("2000Mi"),
					},
				},
				ServiceAccountName: p.name(),
				SecurityContext:    &corev1.PodSecurityContext{RunAsUser: ptr.To(int64(0))},
				Storage: &monitoringv1.StorageSpec{
					VolumeClaimTemplate: monitoringv1.EmbeddedPersistentVolumeClaim{
						EmbeddedObjectMetadata: monitoringv1.EmbeddedObjectMetadata{Name: "prometheus-db"},
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							Resources:   corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: p.values.StorageCapacity}},
						},
					},
				},

				ServiceMonitorSelector: &metav1.LabelSelector{MatchLabels: monitoringutils.Labels(p.values.Name)},
				PodMonitorSelector:     &metav1.LabelSelector{MatchLabels: monitoringutils.Labels(p.values.Name)},
				ProbeSelector:          &metav1.LabelSelector{MatchLabels: monitoringutils.Labels(p.values.Name)},
				ScrapeConfigSelector:   &metav1.LabelSelector{MatchLabels: monitoringutils.Labels(p.values.Name)},

				ServiceMonitorNamespaceSelector: &metav1.LabelSelector{},
				PodMonitorNamespaceSelector:     &metav1.LabelSelector{},
				ProbeNamespaceSelector:          &metav1.LabelSelector{},
				ScrapeConfigNamespaceSelector:   &metav1.LabelSelector{},
			},
			RuleSelector:          &metav1.LabelSelector{MatchLabels: monitoringutils.Labels(p.values.Name)},
			RuleNamespaceSelector: &metav1.LabelSelector{},
		},
	}

	if takeOverOldPV {
		obj.Spec.InitContainers = append(obj.Spec.InitContainers, corev1.Container{
			Name:            "take-over-old-pv",
			Image:           p.values.DataMigration.ImageAlpine,
			ImagePullPolicy: corev1.PullIfNotPresent,
			VolumeMounts:    []corev1.VolumeMount{{Name: "prometheus-db", MountPath: "/prometheus"}},
			Command:         []string{"/bin/sh", "-c"},
			Args:            []string{`if [[ -d /prometheus/prometheus- ]]; then mv /prometheus/prometheus- /prometheus/prometheus-db && echo "rename done"; else echo "rename already done"; fi`},
		})
	}

	return obj
}
