// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package terminal

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"

	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

func (t *terminal) service() *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: t.namespace,
			Labels:    getLabels(),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: getLabels(),
			Ports: []corev1.ServicePort{
				{
					Name:       portNameAdmission,
					Protocol:   corev1.ProtocolTCP,
					Port:       443,
					TargetPort: intstr.FromInt32(portAdmission),
				},
				{
					Name:       portNameMetrics,
					Protocol:   corev1.ProtocolTCP,
					Port:       int32(portMetrics),
					TargetPort: intstr.FromInt32(portMetrics),
				},
			},
		},
	}

	utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForWebhookTargets(svc, networkingv1.NetworkPolicyPort{
		Port:     ptr.To(intstr.FromInt32(portAdmission)),
		Protocol: ptr.To(corev1.ProtocolTCP),
	}))

	utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForGardenScrapeTargets(svc, networkingv1.NetworkPolicyPort{
		Port:     ptr.To(intstr.FromInt32(portMetrics)),
		Protocol: ptr.To(corev1.ProtocolTCP),
	}))

	gardenerutils.ReconcileTopologyAwareRoutingSettings(svc, t.values.TopologyAwareRoutingEnabled, t.values.RuntimeVersion)

	return svc
}
