// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package machinecontrollermanager

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"

	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// ProviderSidecarContainer returns a corev1.Container object which can be injected into the machine-controller-manager
// deployment managed by the gardenlet. This function can be used in provider-specific control plane webhook
// implementations when the standard sidecar container is required.
func ProviderSidecarContainer(namespace, providerName, image string) corev1.Container {
	const metricsPort = 10259
	return corev1.Container{
		Name:            providerSidecarContainerName(providerName),
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command: []string{
			"./machine-controller",
			"--control-kubeconfig=inClusterConfig",
			"--machine-creation-timeout=20m",
			"--machine-drain-timeout=2h",
			"--machine-health-timeout=10m",
			"--machine-safety-apiserver-statuscheck-timeout=30s",
			"--machine-safety-apiserver-statuscheck-period=1m",
			"--machine-safety-orphan-vms-period=30m",
			"--namespace=" + namespace,
			"--port=" + strconv.Itoa(metricsPort),
			"--target-kubeconfig=" + gardenerutils.PathGenericKubeconfig,
			"--v=3",
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/healthz",
					Port:   intstr.FromInt32(metricsPort),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 30,
			TimeoutSeconds:      5,
			PeriodSeconds:       10,
			SuccessThreshold:    1,
			FailureThreshold:    3,
		},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "kubeconfig",
			MountPath: gardenerutils.VolumeMountPathGenericKubeconfig,
			ReadOnly:  true,
		}},
	}
}

// ProviderSidecarVPAContainerPolicy returns a vpaautoscalingv1.ContainerResourcePolicy object which can be injected
// into the machine-controller-manager-vpa VPA managed by the gardenlet. This function can be used in provider-specific
// control plane webhook implementations when the standard container policy for the sidecar is required.
func ProviderSidecarVPAContainerPolicy(providerName string, minAllowed, maxAllowed corev1.ResourceList) vpaautoscalingv1.ContainerResourcePolicy {
	ccv := vpaautoscalingv1.ContainerControlledValuesRequestsOnly
	return vpaautoscalingv1.ContainerResourcePolicy{
		ContainerName:    providerSidecarContainerName(providerName),
		ControlledValues: &ccv,
		MinAllowed:       minAllowed,
		MaxAllowed:       maxAllowed,
	}
}

func providerSidecarContainerName(providerName string) string {
	return "machine-controller-manager-" + providerName
}
