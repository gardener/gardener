// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package machinecontrollermanager_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/component/nodemanagement/machinecontrollermanager"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Provider", func() {
	const (
		provider = "provider-test"
		image    = "provider-test:latest"
	)

	var (
		namespace string
		shoot     *gardencorev1beta1.Shoot

		container corev1.Container
	)

	BeforeEach(func() {
		namespace = "test-namespace"
		shoot = &gardencorev1beta1.Shoot{}
	})

	JustBeforeEach(func() {
		container = corev1.Container{
			Name:            "machine-controller-manager-" + provider,
			Image:           image,
			ImagePullPolicy: "IfNotPresent",
			Args: []string{
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
			Ports: []corev1.ContainerPort{{
				Name:          "providermetrics",
				ContainerPort: 10259,
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
	})

	When("the shoot is not autonomous", func() {
		JustBeforeEach(func() {
			container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
				Name:      "kubeconfig",
				MountPath: "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig",
				ReadOnly:  true,
			})
		})

		It("should return the provider sidecar container", func() {
			Expect(ProviderSidecarContainer(shoot, namespace, provider, image)).To(DeepEqual(container))
		})
	})

	When("the shoot is autonomous", func() {
		BeforeEach(func() {
			shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, gardencorev1beta1.Worker{
				ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
			})
		})

		JustBeforeEach(func() {
			for i, s := range container.Args {
				if strings.HasPrefix(s, "--target-kubeconfig=") {
					container.Args[i] = "--target-kubeconfig="
				}
			}
		})

		When("running the control plane (gardenadm init)", func() {
			BeforeEach(func() {
				namespace = "kube-system"
			})

			It("should return the provider sidecar container", func() {
				Expect(ProviderSidecarContainer(shoot, namespace, provider, image)).To(DeepEqual(container))
			})
		})

		When("not running the control plane (gardenadm bootstrap)", func() {
			It("should return the provider sidecar container", func() {
				Expect(ProviderSidecarContainer(shoot, namespace, provider, image)).To(DeepEqual(container))
			})
		})
	})

	It("should return a default VPA container policy object for the provider-specific sidecar container", func() {
		Expect(ProviderSidecarVPAContainerPolicy(provider)).To(Equal(vpaautoscalingv1.ContainerResourcePolicy{
			ContainerName:    "machine-controller-manager-" + provider,
			ControlledValues: ptr.To(vpaautoscalingv1.ContainerControlledValuesRequestsOnly),
		}))
	})
})
