// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/alertmanager"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
)

func (p *prometheus) prometheus(cortexConfigMap *corev1.ConfigMap) *monitoringv1.Prometheus {
	obj := &monitoringv1.Prometheus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.values.Name,
			Namespace: p.namespace,
			Labels:    p.getLabels(),
		},
		Spec: monitoringv1.PrometheusSpec{
			RetentionSize:      p.values.RetentionSize,
			EvaluationInterval: "1m",
			CommonPrometheusFields: monitoringv1.CommonPrometheusFields{
				ScrapeInterval: "1m",
				ScrapeTimeout:  p.values.ScrapeTimeout,
				ReloadStrategy: ptr.To(monitoringv1.HTTPReloadStrategyType),
				ExternalLabels: p.values.ExternalLabels,
				AdditionalScrapeConfigs: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: p.name() + secretNameSuffixAdditionalScrapeConfigs},
					Key:                  dataKeyAdditionalScrapeConfigs,
				},

				PodMetadata: &monitoringv1.EmbeddedObjectMetadata{
					Labels: utils.MergeStringMaps(map[string]string{
						v1beta1constants.LabelNetworkPolicyToDNS:              v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelObservabilityApplication:        p.name(),
					}, p.values.AdditionalPodLabels),
				},
				PriorityClassName: p.values.PriorityClassName,
				Replicas:          &p.values.Replicas,
				Shards:            ptr.To[int32](1),
				Image:             &p.values.Image,
				ImagePullPolicy:   corev1.PullIfNotPresent,
				Version:           p.values.Version,
				Resources: corev1.ResourceRequirements{
					Requests: ptr.Deref(p.values.ResourceRequests, corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("300m"),
						corev1.ResourceMemory: resource.MustParse("1000Mi"),
					}),
				},
				ServiceAccountName: p.name(),
				SecurityContext:    &corev1.PodSecurityContext{RunAsUser: ptr.To[int64](0)},
				Storage: &monitoringv1.StorageSpec{
					VolumeClaimTemplate: monitoringv1.EmbeddedPersistentVolumeClaim{
						EmbeddedObjectMetadata: monitoringv1.EmbeddedObjectMetadata{Name: "prometheus-db"},
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: p.values.StorageCapacity}},
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
				Web: &monitoringv1.PrometheusWebSpec{
					MaxConnections: ptr.To[int32](1024),
				},
			},
			RuleSelector:          &metav1.LabelSelector{MatchLabels: monitoringutils.Labels(p.values.Name)},
			RuleNamespaceSelector: &metav1.LabelSelector{},
		},
	}

	if p.values.RestrictToNamespace {
		obj.Spec.ServiceMonitorNamespaceSelector = nil
		obj.Spec.PodMonitorNamespaceSelector = nil
		obj.Spec.ProbeNamespaceSelector = nil
		obj.Spec.ScrapeConfigNamespaceSelector = nil
		obj.Spec.RuleNamespaceSelector = nil
	}

	if p.values.Ingress != nil {
		obj.Spec.ExternalURL = "https://" + p.values.Ingress.Host
	}

	if p.values.Retention != nil {
		obj.Spec.Retention = *p.values.Retention
	}

	if p.values.Alerting != nil {
		if len(p.values.Alerting.Alertmanagers) > 0 {
			obj.Spec.Alerting = &monitoringv1.AlertingSpec{}
		}
		for _, alertManager := range p.values.Alerting.Alertmanagers {
			obj.Spec.Alerting.Alertmanagers = append(obj.Spec.Alerting.Alertmanagers, monitoringv1.AlertmanagerEndpoints{
				Namespace: alertManager.Namespace,
				Name:      alertManager.Name,
				Port:      intstr.FromString(alertmanager.PortNameMetrics),
				AlertRelabelConfigs: append(
					[]monitoringv1.RelabelConfig{{
						SourceLabels: []monitoringv1.LabelName{"ignoreAlerts"},
						Regex:        `true`,
						Action:       "drop",
					}},
					p.values.AdditionalAlertRelabelConfigs...,
				),
			})
		}

		if p.values.Alerting.AdditionalAlertmanager != nil {
			obj.Spec.AdditionalAlertManagerConfigs = &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: p.name() + secretNameSuffixAdditionalAlertmanagerConfigs},
				Key:                  dataKeyAdditionalAlertmanagerConfigs,
			}
		}
	}

	if p.values.RemoteWrite != nil {
		spec := monitoringv1.RemoteWriteSpec{URL: p.values.RemoteWrite.URL}

		if len(p.values.RemoteWrite.KeptMetrics) > 0 {
			spec.WriteRelabelConfigs = []monitoringv1.RelabelConfig{monitoringutils.StandardMetricRelabelConfig(p.values.RemoteWrite.KeptMetrics...)[0]}
		}

		if p.values.RemoteWrite.GlobalShootRemoteWriteSecret != nil {
			spec.BasicAuth = &monitoringv1.BasicAuth{
				Username: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: p.name() + secretNameSuffixRemoteWriteBasicAuth},
					Key:                  "username",
				},
				Password: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: p.name() + secretNameSuffixRemoteWriteBasicAuth},
					Key:                  "password",
				},
			}
		}

		obj.Spec.RemoteWrite = append(obj.Spec.RemoteWrite, spec)
	}

	if p.values.Cortex != nil {
		obj.Spec.Containers = append(obj.Spec.Containers, p.cortexContainer())
		obj.Spec.Volumes = append(obj.Spec.Volumes, p.cortexVolume(cortexConfigMap.Name))
	}

	utilruntime.Must(references.InjectAnnotations(obj))
	return obj
}
