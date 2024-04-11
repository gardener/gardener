// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubeobjects

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

func makeService(namespace string) *corev1.Service {
	//This service intentionally does not contain a pod selector. As a result, KCM does not perform any endpoint management.
	//Endpoint management is instead done by the gardener-custom-metrics leader instance, which ensures a single endpoint,
	//directing all traffic to the leader.
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gcmxBaseName,
			Namespace: namespace,
			Annotations: map[string]string{
				resourcesv1alpha1.NetworkingFromWorldToPorts: `[{"protocol":"TCP","port":6443}]`,
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:       443,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt32(6443),
				},
			},
			PublishNotReadyAddresses: true,
			SessionAffinity:          corev1.ServiceAffinityNone,
			Type:                     corev1.ServiceTypeClusterIP,
		},
	}
}
