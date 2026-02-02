// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/utils"
)

func (g *gardenerAPIServer) endpointslice(clusterIP string) *discoveryv1.EndpointSlice {
	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: metav1.NamespaceSystem,
			Labels: utils.MergeStringMaps(GetLabels(), map[string]string{
				discoveryv1.LabelServiceName: serviceName,
			}),
		},
		// TODO(tobschli): This could also be IPv6, depending on the cluster setup.
		AddressType: discoveryv1.AddressTypeIPv4,
		Ports: []discoveryv1.EndpointPort{{
			Port:     ptr.To(int32(servicePort)),
			Protocol: ptr.To(corev1.ProtocolTCP),
		}},
		Endpoints: []discoveryv1.Endpoint{{
			Addresses: []string{clusterIP},
		}},
	}
}
