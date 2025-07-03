// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package podkubeapiserverloadbalancing_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/resourcemanager/webhook/podkubeapiserverloadbalancing"
)

var _ = Describe("Handler", func() {
	var (
		ctx          context.Context
		log          logr.Logger
		targetClient client.Client

		handler *Handler
		pod     *corev1.Pod

		testNamespace  string
		istioNamespace string
	)

	BeforeEach(func() {
		ctx = context.Background()
		log = logr.Discard()

		testNamespace = "foo-namespace"
		istioNamespace = "istio-gateway"

		targetClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		handler = &Handler{Logger: log, TargetClient: targetClient}
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNamespace,
				Name:      "bar-pod",
			},
		}
	})

	Describe("#Default", func() {
		When("the istio-internal-load-balancing configmap is not found", func() {
			It("should not patch the pod", func() {
				Expect(handler.Default(ctx, pod)).To(Succeed())
				Expect(pod.Spec.HostAliases).To(BeEmpty())
				Expect(pod.Labels).To(BeEmpty())
			})
		})

		When("the istio-internal-load-balancing configmap is found", func() {
			BeforeEach(func() {
				configMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNamespace,
						Name:      "istio-internal-load-balancing",
					},
					Data: map[string]string{
						resourcemanagerconfigv1alpha1.HostsConfigMapKey:          "api.example.com,api2.example.com",
						resourcemanagerconfigv1alpha1.IstioNamespaceConfigMapKey: istioNamespace,
					},
				}
				Expect(targetClient.Create(ctx, configMap)).To(Succeed())
				DeferCleanup(func() {
					Expect(targetClient.Delete(ctx, configMap)).To(Succeed())
				})
			})

			When("the pod does not use a generic kubeconfig", func() {
				It("should not patch the pod", func() {
					Expect(handler.Default(ctx, pod)).To(Succeed())
					Expect(pod.Spec.HostAliases).To(BeEmpty())
					Expect(pod.Labels).To(BeEmpty())
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
							Namespace: istioNamespace,
							Name:      "istio-ingressgateway-internal",
						},
						Spec: corev1.ServiceSpec{
							ClusterIP:  "10.10.10.10",
							ClusterIPs: []string{"10.10.10.10", "3fff::2"},
						},
					}
					Expect(targetClient.Create(ctx, istioService)).To(Succeed())
					DeferCleanup(func() {
						Expect(targetClient.Delete(ctx, istioService)).To(Succeed())
					})

					Expect(handler.Default(ctx, pod)).To(Succeed())
					Expect(pod.Spec.HostAliases).To(HaveLen(2))
					Expect(pod.Spec.HostAliases[0].IP).To(Equal("10.10.10.10"))
					Expect(pod.Spec.HostAliases[0].Hostnames).To(ConsistOf("api.example.com", "api2.example.com"))
					Expect(pod.Spec.HostAliases[1].IP).To(Equal("3fff::2"))
					Expect(pod.Spec.HostAliases[1].Hostnames).To(ConsistOf("api.example.com", "api2.example.com"))

					Expect(pod.Labels).To(HaveKeyWithValue(
						"networking.resources.gardener.cloud/to-"+istioNamespace+"-istio-ingressgateway-internal-tcp-9443",
						"allowed",
					))
				})

				It("should fail if the istio-ingressgateway service is not found", func() {
					Expect(handler.Default(ctx, pod)).To(MatchError(ContainSubstring("failed to get internal istio-ingressgateway service")))
				})
			})
		})
	})
})
