// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package machinecontrollermanager_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"

	. "github.com/gardener/gardener/pkg/component/nodemanagement/machinecontrollermanager"
)

var _ = Describe("Provider", func() {
	var provider = "provider-test"

	It("should return a default provider-specific sidecar container object", func() {
		var (
			namespace = "test-namespace"
			image     = "provider-test:latest"
		)

		Expect(ProviderSidecarContainer(namespace, provider, image)).To(Equal(corev1.Container{
			Name:            "machine-controller-manager-" + provider,
			Image:           image,
			ImagePullPolicy: "IfNotPresent",
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
				"--port=10259",
				"--target-kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
				"--v=3",
			},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path:   "/healthz",
						Port:   intstr.FromInt32(10259),
						Scheme: "HTTP",
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
				MountPath: "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig",
				ReadOnly:  true,
			}},
		}))
	})

	It("should return a default VPA container policy object for the provider-specific sidecar container", func() {
		var (
			ccv        = vpaautoscalingv1.ContainerControlledValuesRequestsOnly
			minAllowed = corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1M")}
			maxAllowed = corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("5M")}
		)

		Expect(ProviderSidecarVPAContainerPolicy(provider, minAllowed, maxAllowed)).To(Equal(vpaautoscalingv1.ContainerResourcePolicy{
			ContainerName:    "machine-controller-manager-" + provider,
			ControlledValues: &ccv,
			MinAllowed:       minAllowed,
			MaxAllowed:       maxAllowed,
		}))
	})
})
