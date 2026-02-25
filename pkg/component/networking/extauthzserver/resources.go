// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extauthzserver

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (e *extAuthzServer) getDeployment(volumes []corev1.Volume, volumeMounts []corev1.VolumeMount) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      e.getPrefix() + name,
			Namespace: e.namespace,
			Labels:    e.getLabels(),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: e.getLabels(),
			},
			Replicas: &e.values.Replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: e.getLabels(),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            name,
							Image:           e.values.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{
								"--grpc-reflection",
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt32(Port),
									},
								},
								SuccessThreshold: 1,
								FailureThreshold: 2,
								PeriodSeconds:    10,
								TimeoutSeconds:   5,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt32(Port),
									},
								},
								SuccessThreshold: 1,
								FailureThreshold: 2,
								PeriodSeconds:    10,
								TimeoutSeconds:   5,
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: Port,
									Name:          "grpc",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("5m"),
									corev1.ResourceMemory: resource.MustParse("16Mi"),
								},
							},
							VolumeMounts: volumeMounts,
						},
					},
					PriorityClassName: e.values.PriorityClassName,
					Volumes:           volumes,
				},
			},
		},
	}
}
