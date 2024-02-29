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

package customresources

import (
	"fmt"

	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

// GetFluentBit returns instance of FluentBit custom resource.
func GetFluentBit(labels map[string]string, fluentBitName, namespace, image, initImage, priorityClass string) *fluentbitv1alpha2.FluentBit {
	annotations := map[string]string{
		resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationPrefix + v1beta1constants.LabelNetworkPolicySeedScrapeTargets + resourcesv1alpha1.NetworkPolicyFromPolicyAnnotationSuffix: `[{"port":2020,"protocol":"TCP"},{"port":2021,"protocol":"TCP"}]`,
	}

	return &fluentbitv1alpha2.FluentBit{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v-%v", fluentBitName, utils.ComputeSHA256Hex([]byte(fmt.Sprintf("%v%v", labels, annotations)))[:5]),
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: fluentbitv1alpha2.FluentBitSpec{
			FluentBitConfigName: fluentBitConfigName,
			Image:               image,
			Command: []string{
				"/fluent-bit/bin/fluent-bit-watcher",
				"-e",
				"/fluent-bit/plugins/out_vali.so",
				"-c",
				"/fluent-bit/config/fluent-bit.conf",
			},
			PriorityClassName: priorityClass,
			Ports: []corev1.ContainerPort{
				{
					Name:          "metrics-plugin",
					ContainerPort: 2021,
					Protocol:      "TCP",
				},
			},
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("650Mi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("150m"),
					corev1.ResourceMemory: resource.MustParse("200Mi"),
				},
			},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/api/v1/metrics/prometheus",
						Port: intstr.FromInt32(2020),
					},
				},
				PeriodSeconds: 10,
			},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/healthz",
						Port: intstr.FromInt32(2021),
					},
				},
				PeriodSeconds:       300,
				InitialDelaySeconds: 90,
			},
			Tolerations: []corev1.Toleration{
				{
					Key:    "node-role.kubernetes.io/master",
					Effect: corev1.TaintEffectNoSchedule,
				},
				{
					Key:    "node-role.kubernetes.io/control-plane",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "runlogjournal",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/run/log/journal",
						},
					},
				},
				{
					Name: "varfluent",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/var/fluentbit",
						},
					},
				},
				{
					Name: "plugins",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			VolumesMounts: []corev1.VolumeMount{
				{
					Name:      "runlogjournal",
					MountPath: "/run/log/journal",
				},
				{
					Name:      "varfluent",
					MountPath: "/var/fluentbit",
				},
				{
					Name:      "plugins",
					MountPath: "/fluent-bit/plugins",
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:  "install-plugin",
					Image: initImage,
					Command: []string{
						"cp",
						"/source/plugins/.",
						"/plugins",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "plugins",
							MountPath: "/plugins",
						},
					},
				},
			},
			RBACRules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"extensions.gardener.cloud"},
					Resources: []string{"clusters"},
					Verbs:     []string{"get", "list", "watch"},
				},
			},
			Service: fluentbitv1alpha2.FluentBitService{
				Name:        fluentBitName,
				Annotations: annotations,
				Labels:      labels,
			},
		},
	}
}
