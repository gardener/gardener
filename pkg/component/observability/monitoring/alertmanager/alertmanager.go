// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

func (a *alertManager) alertManager() *monitoringv1.Alertmanager {
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
					v1beta1constants.LabelObservabilityApplication:       a.name(),
				}),
			},
			PriorityClassName: a.values.PriorityClassName,
			Replicas:          &a.values.Replicas,
			Image:             &a.values.Image,
			ImagePullPolicy:   corev1.PullIfNotPresent,
			Version:           a.values.Version,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("20Mi"),
				},
			},
			SecurityContext: &corev1.PodSecurityContext{RunAsUser: ptr.To[int64](0)},
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
			AlertmanagerConfigMatcherStrategy:   monitoringv1.AlertmanagerConfigMatcherStrategy{Type: "None"},
			LogLevel:                            "info",
			ForceEnableClusterMode:              true,
		},
	}

	if a.hasSMTPSecret() {
		obj.Spec.AlertmanagerConfiguration = &monitoringv1.AlertmanagerConfiguration{Name: a.name()}
	}

	if a.values.Replicas > 1 {
		// The `alertmanager-operated` service is automatically created by `prometheus-operator` and not managed by our
		// code. It is used to enable peer/mesh communication when more than 1 replica is used.
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

	return obj
}
