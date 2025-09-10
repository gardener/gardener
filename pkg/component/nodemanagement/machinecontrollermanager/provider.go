// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package machinecontrollermanager

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

const (
	portProviderMetrics     = 10259
	portNameProviderMetrics = "providermetrics"
)

// ProviderSidecarContainer returns a corev1.Container object which can be injected into the machine-controller-manager
// deployment managed by the gardenlet. This function can be used in provider-specific control plane webhook
// implementations when the standard sidecar container is required.
// The shoot object can be read from the `Cluster` object, e.g., using the GardenContext.GetCluster method in webhooks.
func ProviderSidecarContainer(shoot *gardencorev1beta1.Shoot, controlPlaneNamespace, providerName, image string) corev1.Container {
	autonomousShoot := v1beta1helper.IsShootAutonomous(shoot.Spec.Provider.Workers)

	c := corev1.Container{
		Name:            providerSidecarContainerName(providerName),
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Args: []string{
			"--control-kubeconfig=inClusterConfig",
			"--machine-creation-timeout=20m",
			"--machine-drain-timeout=2h",
			"--machine-health-timeout=10m",
			"--machine-safety-apiserver-statuscheck-timeout=30s",
			"--machine-safety-apiserver-statuscheck-period=1m",
			"--machine-safety-orphan-vms-period=30m",
			"--namespace=" + controlPlaneNamespace,
			"--port=" + strconv.Itoa(portProviderMetrics),
			"--target-kubeconfig=" + targetKubeconfig(autonomousShoot, controlPlaneNamespace),
			"--v=3",
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/healthz",
					Port:   intstr.FromInt32(portProviderMetrics),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 30,
			TimeoutSeconds:      5,
			PeriodSeconds:       10,
			SuccessThreshold:    1,
			FailureThreshold:    3,
		},
		Ports: []corev1.ContainerPort{{
			Name:          portNameProviderMetrics,
			ContainerPort: portProviderMetrics,
			Protocol:      corev1.ProtocolTCP,
		}},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("20Mi"),
			},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
		},
	}

	if !autonomousShoot {
		c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
			Name:      "kubeconfig",
			MountPath: gardenerutils.VolumeMountPathGenericKubeconfig,
			ReadOnly:  true,
		})
	}

	return c
}

// ProviderSidecarVPAContainerPolicy returns a vpaautoscalingv1.ContainerResourcePolicy object which can be injected
// into the machine-controller-manager-vpa VPA managed by the gardenlet. This function can be used in provider-specific
// control plane webhook implementations when the standard container policy for the sidecar is required.
func ProviderSidecarVPAContainerPolicy(providerName string) vpaautoscalingv1.ContainerResourcePolicy {
	return vpaautoscalingv1.ContainerResourcePolicy{
		ContainerName:    providerSidecarContainerName(providerName),
		ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
	}
}

func providerSidecarContainerName(providerName string) string {
	return "machine-controller-manager-" + providerName
}
