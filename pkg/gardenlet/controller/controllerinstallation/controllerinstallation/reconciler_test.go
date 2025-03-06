// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerinstallation_test

import (
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

	Describe("#MutateSpecForControlPlaneNodeBootstrapping", func() {
		var (
			pod         *corev1.Pod
			deployment  *appsv1.Deployment
			statefulSet *appsv1.StatefulSet
			daemonSet   *appsv1.DaemonSet
			job         *batchv1.Job
			cronJob     *batchv1.CronJob
		)

		BeforeEach(func() {
			pod = &corev1.Pod{}
			deployment = &appsv1.Deployment{}
			statefulSet = &appsv1.StatefulSet{}
			daemonSet = &appsv1.DaemonSet{}
			job = &batchv1.Job{}
			cronJob = &batchv1.CronJob{}
		})

		It("should not change objects if not bootstrapping control plane node", func() {
			reconciler.BootstrapControlPlaneNode = false

			p := pod.DeepCopy()
			Expect(reconciler.MutateSpecForControlPlaneNodeBootstrapping(p)).To(Succeed())
			Expect(p).To(Equal(pod))

			d := deployment.DeepCopy()
			Expect(reconciler.MutateSpecForControlPlaneNodeBootstrapping(d)).To(Succeed())
			Expect(d).To(Equal(deployment))

			s := statefulSet.DeepCopy()
			Expect(reconciler.MutateSpecForControlPlaneNodeBootstrapping(s)).To(Succeed())
			Expect(s).To(Equal(statefulSet))

			ds := daemonSet.DeepCopy()
			Expect(reconciler.MutateSpecForControlPlaneNodeBootstrapping(ds)).To(Succeed())
			Expect(ds).To(Equal(daemonSet))

			j := job.DeepCopy()
			Expect(reconciler.MutateSpecForControlPlaneNodeBootstrapping(j)).To(Succeed())
			Expect(j).To(Equal(job))

			c := cronJob.DeepCopy()
			Expect(reconciler.MutateSpecForControlPlaneNodeBootstrapping(c)).To(Succeed())
			Expect(c).To(Equal(cronJob))
		})

		It("should adapt objects if bootstrapping control plane node", func() {
			reconciler.BootstrapControlPlaneNode = true

			checkHostNetworkAndTolerations := func(podSpec *corev1.PodSpec) {
				GinkgoHelper()
				Expect(podSpec.HostNetwork).To(BeTrue())
				Expect(podSpec.Tolerations).To(Equal([]corev1.Toleration{
					{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
					{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoExecute},
				}))
			}

			for _, obj := range []struct {
				object    runtime.Object
				checkFunc func()
			}{
				{pod, func() {
					checkHostNetworkAndTolerations(&pod.Spec)
				}},
				{deployment, func() {
					Expect(deployment.Spec.Replicas).To(Equal(ptr.To(int32(1))))
					checkHostNetworkAndTolerations(&deployment.Spec.Template.Spec)
				}},
				{statefulSet, func() {
					checkHostNetworkAndTolerations(&statefulSet.Spec.Template.Spec)
				}},
				{daemonSet, func() {
					checkHostNetworkAndTolerations(&daemonSet.Spec.Template.Spec)
				}},
				{job, func() {
					checkHostNetworkAndTolerations(&job.Spec.Template.Spec)
				}},
				{cronJob, func() {
					checkHostNetworkAndTolerations(&cronJob.Spec.JobTemplate.Spec.Template.Spec)
				}},
			} {
				Expect(reconciler.MutateSpecForControlPlaneNodeBootstrapping(obj.object)).To(Succeed(), "for %T", obj.object)
				obj.checkFunc()
			}
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
