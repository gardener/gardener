// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package gardeneradmissioncontroller

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

func (a *gardenerAdmissionController) service() *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServiceName,
			Namespace: a.namespace,
			Labels:    GetLabels(),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: GetLabels(),
			Ports: []corev1.ServicePort{
				{
					Name:       "https",
					Protocol:   corev1.ProtocolTCP,
					Port:       443,
					TargetPort: intstr.FromInt32(serverPort),
				},
				{
					Name:       "metrics",
					Protocol:   corev1.ProtocolTCP,
					Port:       int32(metricsPort),
					TargetPort: intstr.FromInt32(metricsPort),
				},
			},
		},
	}

	utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForWebhookTargets(svc, networkingv1.NetworkPolicyPort{
		Port:     utils.IntStrPtrFromInt32(serverPort),
		Protocol: utils.ProtocolPtr(corev1.ProtocolTCP),
	}))

	gardenerutils.ReconcileTopologyAwareRoutingMetadata(svc, a.values.TopologyAwareRoutingEnabled, a.values.RuntimeVersion)

	return svc
}
