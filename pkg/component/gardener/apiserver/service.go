// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

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
	serviceName = DeploymentName
	servicePort = 443
)

func (g *gardenerAPIServer) serviceRuntime() *corev1.Service {
	service := g.service()
	service.Namespace = g.namespace

	gardenerutils.ReconcileTopologyAwareRoutingSettings(service, g.values.TopologyAwareRoutingEnabled, g.values.RuntimeVersion)
	// allow gardener-apiserver being reached from kube-apiserver
	utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForWebhookTargets(service, networkingv1.NetworkPolicyPort{
		Port:     ptr.To(intstr.FromInt32(port)),
		Protocol: ptr.To(corev1.ProtocolTCP),
	}))

	utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForGardenScrapeTargets(service, networkingv1.NetworkPolicyPort{
		Port:     ptr.To(intstr.FromInt32(port)),
		Protocol: ptr.To(corev1.ProtocolTCP),
	}))

	return service
}

func (g *gardenerAPIServer) service() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: metav1.NamespaceSystem,
			Labels:    GetLabels(),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: GetLabels(),
			Ports: []corev1.ServicePort{{
				Port:       servicePort,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt32(port),
			}},
		},
	}
}
