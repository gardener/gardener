// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerinstallation_test

import (
	"fmt"
	"math"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation/controllerinstallation"
)

var _ = Describe("Reconciler", func() {
	var (
		reconciler *Reconciler
	)

	BeforeEach(func() {
		reconciler = &Reconciler{}
	})

	Describe("#MutateSpecForSelfHostedShootExtensions", func() {
		var (
			pod         *corev1.Pod
			deployment  *appsv1.Deployment
			statefulSet *appsv1.StatefulSet
			daemonSet   *appsv1.DaemonSet
			job         *batchv1.Job
			cronJob     *batchv1.CronJob
		)

		BeforeEach(func() {
			podSpec := corev1.PodSpec{
				InitContainers: []corev1.Container{{}},
				Containers:     []corev1.Container{{}},
			}

			pod = &corev1.Pod{Spec: podSpec}
			deployment = &appsv1.Deployment{Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: podSpec}}}
			statefulSet = &appsv1.StatefulSet{Spec: appsv1.StatefulSetSpec{Template: corev1.PodTemplateSpec{Spec: podSpec}}}
			daemonSet = &appsv1.DaemonSet{Spec: appsv1.DaemonSetSpec{Template: corev1.PodTemplateSpec{Spec: podSpec}}}
			job = &batchv1.Job{Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: podSpec}}}
			cronJob = &batchv1.CronJob{Spec: batchv1.CronJobSpec{JobTemplate: batchv1.JobTemplateSpec{Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: podSpec}}}}}
		})

		It("should not change objects if not responsible for self-hosted shoots", func() {
			reconciler.ForSelfHostedShoot = false

			p := pod.DeepCopy()
			Expect(reconciler.MutateSpecForSelfHostedShootExtensions(p)).To(Succeed())
			Expect(p).To(Equal(pod))

			d := deployment.DeepCopy()
			Expect(reconciler.MutateSpecForSelfHostedShootExtensions(d)).To(Succeed())
			Expect(d).To(Equal(deployment))

			s := statefulSet.DeepCopy()
			Expect(reconciler.MutateSpecForSelfHostedShootExtensions(s)).To(Succeed())
			Expect(s).To(Equal(statefulSet))

			ds := daemonSet.DeepCopy()
			Expect(reconciler.MutateSpecForSelfHostedShootExtensions(ds)).To(Succeed())
			Expect(ds).To(Equal(daemonSet))

			j := job.DeepCopy()
			Expect(reconciler.MutateSpecForSelfHostedShootExtensions(j)).To(Succeed())
			Expect(j).To(Equal(job))

			c := cronJob.DeepCopy()
			Expect(reconciler.MutateSpecForSelfHostedShootExtensions(c)).To(Succeed())
			Expect(c).To(Equal(cronJob))
		})

		When("responsible for self-hosted shoots", func() {
			var (
				checkPodSpec = func(podSpec *corev1.PodSpec, bootstrapControlPlaneNode bool) {
					GinkgoHelper()

					if bootstrapControlPlaneNode {
						Expect(podSpec.HostNetwork).To(BeTrue())

						for _, container := range podSpec.InitContainers {
							Expect(container.Env).To(ContainElement(corev1.EnvVar{Name: "KUBERNETES_SERVICE_HOST", Value: "localhost"}), fmt.Sprintf("init container %s should have KUBERNETES_SERVICE_HOST env var", container.Name))
						}
						for _, container := range podSpec.Containers {
							Expect(container.Env).To(ContainElement(corev1.EnvVar{Name: "KUBERNETES_SERVICE_HOST", Value: "localhost"}), fmt.Sprintf("container %s should have KUBERNETES_SERVICE_HOST env var", container.Name))
						}
					}

					Expect(podSpec.Tolerations).To(Equal([]corev1.Toleration{
						{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
						{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoExecute},
					}))
				}

				assert = func(bootstrapControlPlaneNode bool) {
					for _, obj := range []struct {
						object    runtime.Object
						checkFunc func()
					}{
						{pod, func() {
							checkPodSpec(&pod.Spec, bootstrapControlPlaneNode)
						}},
						{deployment, func() {
							if bootstrapControlPlaneNode {
								Expect(deployment.Spec.Replicas).To(Equal(ptr.To(int32(1))))
								Expect(deployment.Spec.Strategy.Type).To(Equal(appsv1.RecreateDeploymentStrategyType))
								Expect(deployment.Spec.Strategy.RollingUpdate).To(BeNil())
							}
							checkPodSpec(&deployment.Spec.Template.Spec, bootstrapControlPlaneNode)
						}},
						{statefulSet, func() {
							checkPodSpec(&statefulSet.Spec.Template.Spec, bootstrapControlPlaneNode)
						}},
						{daemonSet, func() {
							checkPodSpec(&daemonSet.Spec.Template.Spec, bootstrapControlPlaneNode)
						}},
						{job, func() {
							checkPodSpec(&job.Spec.Template.Spec, bootstrapControlPlaneNode)
						}},
						{cronJob, func() {
							checkPodSpec(&cronJob.Spec.JobTemplate.Spec.Template.Spec, bootstrapControlPlaneNode)
						}},
					} {
						Expect(reconciler.MutateSpecForSelfHostedShootExtensions(obj.object)).To(Succeed(), "for %T", obj.object)
						obj.checkFunc()
					}
				}
			)

			BeforeEach(func() {
				reconciler.ForSelfHostedShoot = true
			})

			When("BootstrapControlPlaneNode is true", func() {
				BeforeEach(func() {
					reconciler.BootstrapControlPlaneNode = true
				})

				It("should mutate the objects", func() {
					assert(reconciler.BootstrapControlPlaneNode)
				})
			})

			When("BootstrapControlPlaneNode is false", func() {
				BeforeEach(func() {
					reconciler.BootstrapControlPlaneNode = false
				})

				It("should mutate the objects", func() {
					assert(reconciler.BootstrapControlPlaneNode)
				})
			})
		})
	})

	Describe("#CalculateUsablePorts", func() {
		It("should calculate usable ports range", func() {
			ports, err := reconciler.CalculateUsablePorts()
			Expect(err).To(Succeed())
			Expect(ports).To(HaveLen(5))
			allocatedPorts := map[int]struct{}{}
			for _, p := range ports {
				Expect(p).To(BeNumerically(">", 0))
				Expect(p).To(BeNumerically("<", math.MaxUint16+1))
				Expect(allocatedPorts).NotTo(HaveKey(p))
				allocatedPorts[p] = struct{}{}
			}
		})

		It("should not have any overlap between two calls", func() {
			ports, err := reconciler.CalculateUsablePorts()
			Expect(err).To(Succeed())
			allocatedPorts := map[int]struct{}{}
			for _, p := range ports {
				Expect(allocatedPorts).NotTo(HaveKey(p))
				allocatedPorts[p] = struct{}{}
			}
			ports, err = reconciler.CalculateUsablePorts()
			Expect(err).To(Succeed())
			for _, p := range ports {
				Expect(allocatedPorts).NotTo(HaveKey(p))
			}
		})
	})
})
