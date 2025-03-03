// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerinstallation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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

	Describe("#BootstrapControlPlaneNodeFunc", func() {
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
			Expect(reconciler.BootstrapControlPlaneNodeFunc(p)).To(Succeed())
			Expect(p).To(Equal(pod))

			d := deployment.DeepCopy()
			Expect(reconciler.BootstrapControlPlaneNodeFunc(d)).To(Succeed())
			Expect(d).To(Equal(deployment))

			s := statefulSet.DeepCopy()
			Expect(reconciler.BootstrapControlPlaneNodeFunc(s)).To(Succeed())
			Expect(s).To(Equal(statefulSet))

			ds := daemonSet.DeepCopy()
			Expect(reconciler.BootstrapControlPlaneNodeFunc(ds)).To(Succeed())
			Expect(ds).To(Equal(daemonSet))

			j := job.DeepCopy()
			Expect(reconciler.BootstrapControlPlaneNodeFunc(j)).To(Succeed())
			Expect(j).To(Equal(job))

			c := cronJob.DeepCopy()
			Expect(reconciler.BootstrapControlPlaneNodeFunc(c)).To(Succeed())
			Expect(c).To(Equal(cronJob))
		})

		It("should adapt objects if bootstrapping control plane node", func() {
			reconciler.BootstrapControlPlaneNode = true

			Expect(reconciler.BootstrapControlPlaneNodeFunc(pod)).To(Succeed())
			Expect(pod.Spec.HostNetwork).To(BeTrue())
			Expect(pod.Spec.Tolerations).To(Equal([]corev1.Toleration{
				{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
				{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoExecute},
			}))

			Expect(reconciler.BootstrapControlPlaneNodeFunc(deployment)).To(Succeed())
			Expect(deployment.Spec.Replicas).To(Equal(ptr.To(int32(1))))
			Expect(deployment.Spec.Template.Spec.HostNetwork).To(BeTrue())
			Expect(deployment.Spec.Template.Spec.Tolerations).To(Equal([]corev1.Toleration{
				{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
				{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoExecute},
			}))

			Expect(reconciler.BootstrapControlPlaneNodeFunc(statefulSet)).To(Succeed())
			Expect(statefulSet.Spec.Template.Spec.HostNetwork).To(BeTrue())
			Expect(statefulSet.Spec.Template.Spec.Tolerations).To(Equal([]corev1.Toleration{
				{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
				{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoExecute},
			}))

			Expect(reconciler.BootstrapControlPlaneNodeFunc(daemonSet)).To(Succeed())
			Expect(daemonSet.Spec.Template.Spec.HostNetwork).To(BeTrue())
			Expect(daemonSet.Spec.Template.Spec.Tolerations).To(Equal([]corev1.Toleration{
				{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
				{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoExecute},
			}))

			Expect(reconciler.BootstrapControlPlaneNodeFunc(job)).To(Succeed())
			Expect(job.Spec.Template.Spec.HostNetwork).To(BeTrue())
			Expect(job.Spec.Template.Spec.Tolerations).To(Equal([]corev1.Toleration{
				{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
				{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoExecute},
			}))

			Expect(reconciler.BootstrapControlPlaneNodeFunc(cronJob)).To(Succeed())
			Expect(cronJob.Spec.JobTemplate.Spec.Template.Spec.HostNetwork).To(BeTrue())
			Expect(cronJob.Spec.JobTemplate.Spec.Template.Spec.Tolerations).To(Equal([]corev1.Toleration{
				{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
				{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoExecute},
			}))
		})
	})

	Describe("#CalculateNextUsablePorts", func() {
		It("should calculate the first usable ports range", func() {
			ports := reconciler.CalculateNextUsablePorts()
			Expect(ports).To(Equal([]int{10101, 10102, 10103, 10104, 10105}))
		})

		It("should calculate the second usable ports range", func() {
			_ = reconciler.CalculateNextUsablePorts()
			ports := reconciler.CalculateNextUsablePorts()
			Expect(ports).To(Equal([]int{10106, 10107, 10108, 10109, 10110}))
		})
	})
})
