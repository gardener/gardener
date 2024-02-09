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

package prometheusoperator

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
)

const deploymentName = "prometheus-operator"

func (p *prometheusOperator) deployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: p.namespace,
			Labels:    GetLabels(),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:             ptr.To(int32(1)),
			RevisionHistoryLimit: ptr.To(int32(2)),
			Selector:             &metav1.LabelSelector{MatchLabels: GetLabels()},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: utils.MergeStringMaps(GetLabels(), map[string]string{
						v1beta1constants.LabelNetworkPolicyToDNS:              v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
					}),
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccountName,
					PriorityClassName:  p.values.PriorityClassName,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot:   ptr.To(true),
						RunAsUser:      ptr.To(int64(65532)),
						SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
					},
					Containers: []corev1.Container{
						{
							Name:            "prometheus-operator",
							Image:           p.values.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{
								fmt.Sprintf("--prometheus-config-reloader=%s", p.values.ImageConfigReloader),
								"--config-reloader-cpu-request=10m",
								"--config-reloader-cpu-limit=0",
								"--config-reloader-memory-request=25Mi",
								"--config-reloader-memory-limit=50Mi",
								"--enable-config-reloader-probes=false",
							},
							Env: []corev1.EnvVar{{
								Name:  "GOGC",
								Value: "30",
							}},
							Resources: corev1.ResourceRequirements{
								Requests: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("100Mi"),
								},
							},
							Ports: []corev1.ContainerPort{{
								Name:          portName,
								ContainerPort: 8080,
							}},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
								ReadOnlyRootFilesystem:   ptr.To(true),
							},
						},
					},
				},
			},
		},
	}
}
