// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

func (p *prometheus) service() *corev1.Service {
	var targetPort int32 = port
	if p.values.Cortex != nil {
		targetPort = portCortex
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.name(),
			Namespace: p.namespace,
			Labels:    p.getLabels(),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: map[string]string{"prometheus": p.values.Name},
			Ports: []corev1.ServicePort{{
				Name:       ServicePortName,
				Port:       servicePort,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt32(targetPort),
			}},
		},
	}

	utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForSeedScrapeTargets(service, networkingv1.NetworkPolicyPort{
		Port:     ptr.To(intstr.FromInt32(port)),
		Protocol: ptr.To(corev1.ProtocolTCP),
	}))
	utilruntime.Must(gardenerutils.InjectNetworkPolicyNamespaceSelectors(service, metav1.LabelSelector{MatchLabels: map[string]string{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot,
	}}))

	return service
}
