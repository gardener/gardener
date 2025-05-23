// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"

	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

const (
	serviceName     = DeploymentName
	portNameMetrics = "metrics"
)

func (g *gardenerScheduler) service() *corev1.Service {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: g.namespace,
			Labels:    GetLabels(),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: GetLabels(),
			Ports: []corev1.ServicePort{{
				Name:       portNameMetrics,
				Port:       int32(metricsPort),
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt32(metricsPort),
			}},
		},
	}

	utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForGardenScrapeTargets(service, networkingv1.NetworkPolicyPort{
		Port:     ptr.To(intstr.FromInt32(metricsPort)),
		Protocol: ptr.To(corev1.ProtocolTCP),
	}))

	return service
}
