// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package podkubeapiserverloadbalancing_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
)

var _ = Describe("PodKubeAPIServerLoadBalancing tests", func() {
	var pod *corev1.Pod

	BeforeEach(func() {
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-",
				Namespace:    testNamespace.Name,
				Labels: map[string]string{
					"networking.resources.gardener.cloud/to-kube-apiserver-tcp-443": "allowed",
				},
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

		DeferCleanup(func() {
			if pod.Name != "" {
				Expect(testClient.Delete(ctx, pod)).To(Succeed())
			}
		})
	})

	When("the istio-internal-load-balancing configmap is not found", func() {
		It("should not patch the pod", func() {
			Expect(testClient.Create(ctx, pod)).To(Succeed())
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
			Expect(pod.Spec.HostAliases).To(BeEmpty())
			Expect(pod.Labels).To(Equal(map[string]string{"networking.resources.gardener.cloud/to-kube-apiserver-tcp-443": "allowed"}))
		})
	})

	When("the istio-internal-load-balancing configmap is found", func() {
		BeforeEach(func() {
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNamespace.Name,
					Name:      "istio-internal-load-balancing",
				},
				Data: map[string]string{
					resourcemanagerconfigv1alpha1.HostsConfigMapKey:          "api.example.com,api2.example.com",
					resourcemanagerconfigv1alpha1.IstioNamespaceConfigMapKey: testNamespace.Name,
				},
			}
			Expect(testClient.Create(ctx, configMap)).To(Succeed())
			DeferCleanup(func() {
				Expect(testClient.Delete(ctx, configMap)).To(Succeed())
			})
		})

		When("the pod does not use a generic kubeconfig", func() {
			It("should not patch the pod", func() {
				Expect(testClient.Create(ctx, pod)).To(Succeed())
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
				Expect(pod.Spec.HostAliases).To(BeEmpty())
				Expect(pod.Labels).To(Equal(map[string]string{"networking.resources.gardener.cloud/to-kube-apiserver-tcp-443": "allowed"}))
			})
		})

		When("the pod uses a generic kubeconfig", func() {
			BeforeEach(func() {
				pod.Spec.Volumes = []corev1.Volume{
					{
						Name: "foobar",
						VolumeSource: corev1.VolumeSource{
							Projected: &corev1.ProjectedVolumeSource{
								Sources: []corev1.VolumeProjection{
									{Secret: &corev1.SecretProjection{LocalObjectReference: corev1.LocalObjectReference{Name: "generic-token-kubeconfig-foobar"}}},
								},
							},
						},
					},
				}
			})

			It("should add host aliases and network policy label to the pod", func() {
				istioService := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace.Name,
						Name:      "istio-ingressgateway-internal",
					},
					Spec: corev1.ServiceSpec{
						ClusterIP:  "10.0.0.10",
						ClusterIPs: []string{"10.0.0.10"},
						Ports:      []corev1.ServicePort{{Name: "tcp", Port: 9443, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt32(9443)}},
					},
				}
				Expect(testClient.Create(ctx, istioService)).To(Succeed())
				DeferCleanup(func() {
					Expect(testClient.Delete(ctx, istioService)).To(Succeed())
				})

				Expect(testClient.Create(ctx, pod)).To(Succeed())
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
				Expect(pod.Spec.HostAliases).To(HaveLen(1))
				Expect(pod.Spec.HostAliases[0].IP).To(Equal("10.0.0.10"))
				Expect(pod.Spec.HostAliases[0].Hostnames).To(ConsistOf("api.example.com", "api2.example.com"))

				Expect(pod.Labels).To(HaveKeyWithValue(
					"networking.resources.gardener.cloud/to-all-istio-ingresses-istio-ingressgateway-internal-tcp-9443",
					"allowed",
				))
			})

			It("should not patch the pod if it has no access to kube-apiserver", func() {
				pod.Labels = nil
				Expect(testClient.Create(ctx, pod)).To(Succeed())
				Expect(testClient.Get(ctx, client.ObjectKeyFromObject(pod), pod)).To(Succeed())
				Expect(pod.Spec.HostAliases).To(BeEmpty())
				Expect(pod.Labels).To(BeEmpty())
			})

			It("should fail if the istio-ingressgateway service is not found", func() {
				Expect(testClient.Create(ctx, pod)).To(MatchError(ContainSubstring("admission webhook \"pod-kube-apiserver-load-balancing.resources.gardener.cloud\" denied the request: services \"istio-ingressgateway-internal\" not found")))
			})
		})
	})
})
