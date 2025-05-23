// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetesservicehost_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("KubernetesServiceHost tests", func() {
	var pod *corev1.Pod

	BeforeEach(func() {
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    testNamespace.Name,
			},
			Spec: corev1.PodSpec{
				InitContainers: []corev1.Container{
					{
						Name:  "foo-container",
						Image: "foo",
					},
				},
				Containers: []corev1.Container{
					{
						Name:  "bar-container",
						Image: "bar",
					},
				},
			},
		}
	})

	AfterEach(func() {
		Expect(testClient.Delete(ctx, pod)).To(Succeed())
	})

	It("should mutate the pod and inject the environment variable when it is not present yet", func() {
		Expect(testClient.Create(ctx, pod)).To(Succeed())

		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
		Expect(pod.Spec.InitContainers[0].Env).To(ConsistOf(corev1.EnvVar{Name: "KUBERNETES_SERVICE_HOST", Value: host}))
		Expect(pod.Spec.Containers[0].Env).To(ConsistOf(corev1.EnvVar{Name: "KUBERNETES_SERVICE_HOST", Value: host}))
	})

	It("should not mutate the pod when the containers already have the environment variable", func() {
		existingEnvVar := corev1.EnvVar{Name: "KUBERNETES_SERVICE_HOST", Value: "already-set"}

		pod.Spec.InitContainers[0].Env = append(pod.Spec.InitContainers[0].Env, existingEnvVar)
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, existingEnvVar)
		Expect(testClient.Create(ctx, pod)).To(Succeed())

		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
		Expect(pod.Spec.InitContainers[0].Env).To(ConsistOf(existingEnvVar))
		Expect(pod.Spec.Containers[0].Env).To(ConsistOf(existingEnvVar))
	})

	It("should not mutate the pod when the pod is labeled with 'inject=disable'", func() {
		metav1.SetMetaDataLabel(&pod.ObjectMeta, "apiserver-proxy.networking.gardener.cloud/inject", "disable")

		Expect(testClient.Create(ctx, pod)).To(Succeed())

		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
		Expect(pod.Spec.InitContainers[0].Env).To(BeEmpty())
		Expect(pod.Spec.Containers[0].Env).To(BeEmpty())
	})

	It("should not mutate the pod when the namespace is labeled with 'inject=disable'", func() {
		metav1.SetMetaDataLabel(&testNamespace.ObjectMeta, "apiserver-proxy.networking.gardener.cloud/inject", "disable")
		Expect(testClient.Update(ctx, testNamespace)).To(Succeed())
		DeferCleanup(func() {
			delete(testNamespace.Labels, "apiserver-proxy.networking.gardener.cloud/inject")
			Expect(testClient.Update(ctx, testNamespace)).To(Succeed())
		})

		Eventually(func(g Gomega) string {
			g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(testNamespace), testNamespace)).To(Succeed())
			return testNamespace.Labels["apiserver-proxy.networking.gardener.cloud/inject"]
		}).Should((Equal("disable")))

		Expect(testClient.Create(ctx, pod)).To(Succeed())

		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
		Expect(pod.Spec.InitContainers[0].Env).To(BeEmpty())
		Expect(pod.Spec.Containers[0].Env).To(BeEmpty())
	})
})
