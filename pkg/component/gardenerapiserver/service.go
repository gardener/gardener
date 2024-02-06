// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gardenerapiserver

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

const (
	serviceName = DeploymentName
	servicePort = 443
)

func (g *gardenerAPIServer) serviceRuntime() *corev1.Service {
	service := g.service()
	service.Namespace = g.namespace

	gardenerutils.ReconcileTopologyAwareRoutingMetadata(service, g.values.TopologyAwareRoutingEnabled, g.values.RuntimeVersion)
	// allow gardener-apiserver being reached from kube-apiserver
	utilruntime.Must(gardenerutils.InjectNetworkPolicyAnnotationsForWebhookTargets(service, networkingv1.NetworkPolicyPort{
		Port:     utils.IntStrPtrFromInt32(port),
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
