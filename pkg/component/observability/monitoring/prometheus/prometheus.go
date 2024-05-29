// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/alertmanager"
	shootprometheus "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/utils"
)

func (p *prometheus) prometheus(takeOverOldPV bool, cortexConfigMap *corev1.ConfigMap) (*monitoringv1.Prometheus, error) {
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
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("300m"),
						corev1.ResourceMemory: resource.MustParse("1000Mi"),
					},
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
		for _, alertmanagerItem := range p.values.Alerting.Alertmanagers {
			namespace := p.namespace
			if alertmanagerItem.Namespace != nil {
				namespace = *alertmanagerItem.Namespace
			}
			obj.Spec.Alerting.Alertmanagers = append(obj.Spec.Alerting.Alertmanagers,
				monitoringv1.AlertmanagerEndpoints{
					Namespace: namespace,
					Name:      alertmanagerItem.Name,
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

	// TODO(rfranzke): Remove this if-block after v1.100 has been released.
	// For backwards-compatibility with extensions, we still have to mount the shoot CA and access token to the previous
	// paths in the pod so that their scrape config still works.
	if p.values.Name == "shoot" && p.values.Ingress.SecretsManager != nil {
		caSecret, found := p.values.Ingress.SecretsManager.Get(v1beta1constants.SecretNameCACluster)
		if !found {
			return nil, fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
		}
		etcdCASecret, found := p.values.Ingress.SecretsManager.Get(v1beta1constants.SecretNameCAETCD)
		if !found {
			return nil, fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAETCD)
		}
		etcdClientSecret, found := p.values.Ingress.SecretsManager.Get("etcd-client")
		if !found {
			return nil, fmt.Errorf("secret %q not found", "etcd-client")
		}

		const (
			volumeNameShootCA       = "shoot-ca"
			volumeNameShootAccess   = "shoot-access"
			volumeNameEtcdCA        = "ca-etcd"
			volumeNameEtcdClientTLS = "etcd-client-tls"
		)

		obj.Spec.Volumes = append(obj.Spec.Volumes,
			corev1.Volume{
				Name: volumeNameShootCA,
				VolumeSource: corev1.VolumeSource{Projected: &corev1.ProjectedVolumeSource{Sources: []corev1.VolumeProjection{{
					Secret: &corev1.SecretProjection{
						LocalObjectReference: corev1.LocalObjectReference{Name: caSecret.Name},
						// For backwards-compatibility, we make the CA bundle available under both ca.crt and bundle.crt keys.
						Items: []corev1.KeyToPath{
							{Key: "bundle.crt", Path: "bundle.crt"},
							{Key: "bundle.crt", Path: "ca.crt"},
						},
						Optional: ptr.To(false),
					},
				}}}},
			},
			corev1.Volume{
				Name:         volumeNameShootAccess,
				VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: shootprometheus.AccessSecretName}},
			},
			corev1.Volume{
				Name:         volumeNameEtcdCA,
				VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: etcdCASecret.Name}},
			},
			corev1.Volume{
				Name:         volumeNameEtcdClientTLS,
				VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: etcdClientSecret.Name}},
			},
		)
		obj.Spec.VolumeMounts = append(obj.Spec.VolumeMounts,
			corev1.VolumeMount{Name: volumeNameShootCA, MountPath: "/etc/prometheus/seed"},
			corev1.VolumeMount{Name: volumeNameShootAccess, MountPath: "/var/run/secrets/gardener.cloud/shoot/token"},
			corev1.VolumeMount{Name: volumeNameEtcdCA, MountPath: "/srv/kubernetes/etcd/ca"},
			corev1.VolumeMount{Name: volumeNameEtcdClientTLS, MountPath: "/srv/kubernetes/etcd/client"},
		)
	}

	if takeOverOldPV {
		var (
			mountPath = "/prometheus"
			subPath   = ptr.Deref(p.values.DataMigration.OldSubPath, "prometheus-")
			oldDBPath = mountPath + `/` + subPath
			newDBPath = mountPath + `/prometheus-db`
			arg       = `if [[ -d ` + oldDBPath + ` ]]; then mv ` + oldDBPath + ` ` + newDBPath + ` && echo "rename done"; else echo "rename already done"; fi`
		)

		if subPath == "/" {
			arg = `if [[ -d ` + mountPath + `/wal ]]; then rm -rf ` + newDBPath + `; mkdir -p ` + newDBPath + `; find ` + mountPath + ` -mindepth 1 -maxdepth 1 ! -name prometheus-db -exec mv {} ` + newDBPath + ` \; && echo "rename done"; else echo "rename already done"; fi`
		}

		obj.Spec.InitContainers = append(obj.Spec.InitContainers, corev1.Container{
			Name:            "take-over-old-pv",
			Image:           p.values.DataMigration.ImageAlpine,
			ImagePullPolicy: corev1.PullIfNotPresent,
			VolumeMounts:    []corev1.VolumeMount{{Name: "prometheus-db", MountPath: mountPath}},
			Command:         []string{"/bin/sh", "-c"},
			Args:            []string{arg},
		})
	}

	if p.values.Cortex != nil {
		obj.Spec.Containers = append(obj.Spec.Containers, p.cortexContainer())
		obj.Spec.Volumes = append(obj.Spec.Volumes, p.cortexVolume(cortexConfigMap.Name))
	}

	return obj, nil
}
