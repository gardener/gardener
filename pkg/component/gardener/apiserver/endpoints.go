// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (g *gardenerAPIServer) endpoints(clusterIP string) *corev1.Endpoints {
	return &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: metav1.NamespaceSystem,
			Labels:    GetLabels(),
		},
		Subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Port:     servicePort,
				Protocol: corev1.ProtocolTCP,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP: clusterIP,
			}},
		}},
	}
}
