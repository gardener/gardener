// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package podschedulername_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("PodSchedulerName tests", func() {
	var pod *corev1.Pod

	BeforeEach(func() {
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    testNamespace.Name,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "foo-container",
						Image: "foo",
					},
				},
			},
		}
	})

	AfterEach(func() {
		Expect(testClient.Delete(ctx, pod)).To(Succeed())
	})

	It("should not patch the scheduler name when the pod specifies custom scheduler", func() {
		pod.Spec.SchedulerName = "bar-scheduler"
		Expect(testClient.Create(ctx, pod)).To(Succeed())

		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
		Expect(pod.Spec.SchedulerName).To(Equal("bar-scheduler"))
	})

	It("should patch the scheduler name when the pod scheduler is not specified", func() {
		pod.Spec.SchedulerName = ""
		Expect(testClient.Create(ctx, pod)).To(Succeed())

		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
		Expect(pod.Spec.SchedulerName).To(Equal("bin-packing-scheduler"))
	})

	It("should patch the scheduler name when the pod scheduler is 'default-scheduler'", func() {
		pod.Spec.SchedulerName = corev1.DefaultSchedulerName
		Expect(testClient.Create(ctx, pod)).To(Succeed())

		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
		Expect(pod.Spec.SchedulerName).To(Equal("bin-packing-scheduler"))
	})
})
