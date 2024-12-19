// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("TopologyAwareRouting", func() {
	Describe("#ReconcileTopologyAwareRouting", func() {
		Context("when K8s version >= 1.31", func() {
			It("should add the setting in service spec when topology-aware routing is enabled", func() {
				service := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"service.kubernetes.io/topology-aware-hints": "auto",
							"service.kubernetes.io/topology-mode":        "auto",
						},
						Labels: map[string]string{"endpoint-slice-hints.resources.gardener.cloud/consider": "true"},
					},
				}

				ReconcileTopologyAwareRouting(service, true, semver.MustParse("1.31.1"))

				Expect(service.Annotations).NotTo(HaveKey("service.kubernetes.io/topology-aware-hints"))
				Expect(service.Annotations).NotTo(HaveKey("service.kubernetes.io/topology-mode"))
				Expect(service.Labels).NotTo(HaveKey("endpoint-slice-hints.resources.gardener.cloud/consider"))

				Expect(service.Spec.TrafficDistribution).NotTo(BeNil())
				Expect(ptr.Deref(service.Spec.TrafficDistribution, "")).To(Equal(corev1.ServiceTrafficDistributionPreferClose))

			})
		})

		Context("when K8s version < 1.27", func() {
			It("should add the required annotation and label when topology-aware routing is enabled", func() {
				service := &corev1.Service{}

				ReconcileTopologyAwareRouting(service, true, semver.MustParse("1.26.1"))

				Expect(service.Annotations).To(HaveKeyWithValue("service.kubernetes.io/topology-aware-hints", "auto"))
				Expect(service.Labels).To(HaveKeyWithValue("endpoint-slice-hints.resources.gardener.cloud/consider", "true"))
				Expect(service.Annotations).NotTo(HaveKey("service.kubernetes.io/topology-mode"))
				Expect(service.Spec.TrafficDistribution).To(BeNil())
			})
		})

		Context("when K8s version >= 1.27", func() {
			It("should add the required annotation and label when topology-aware routing is enabled", func() {
				service := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"service.kubernetes.io/topology-aware-hints": "auto",
						},
					},
				}

				ReconcileTopologyAwareRouting(service, true, semver.MustParse("1.27.1"))

				Expect(service.Annotations).To(HaveKeyWithValue("service.kubernetes.io/topology-mode", "auto"))
				Expect(service.Annotations).NotTo(HaveKey("service.kubernetes.io/topology-aware-hints"))
				Expect(service.Labels).To(HaveKeyWithValue("endpoint-slice-hints.resources.gardener.cloud/consider", "true"))
				Expect(service.Spec.TrafficDistribution).To(BeNil())
			})
		})

		It("should remove the annotations, label and TrafficDistribution setting when topology-aware routing is disabled", func() {
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"service.kubernetes.io/topology-aware-hints": "auto",
						"service.kubernetes.io/topology-mode":        "auto",
					},
					Labels: map[string]string{"endpoint-slice-hints.resources.gardener.cloud/consider": "true"},
				},
				Spec: corev1.ServiceSpec{TrafficDistribution: ptr.To(corev1.ServiceTrafficDistributionPreferClose)},
			}

			ReconcileTopologyAwareRouting(service, false, semver.MustParse("1.25.1"))

			Expect(service.Annotations).NotTo(HaveKey("service.kubernetes.io/topology-aware-hints"))
			Expect(service.Annotations).NotTo(HaveKey("service.kubernetes.io/topology-mode"))
			Expect(service.Labels).NotTo(HaveKey("endpoint-slice-hints.resources.gardener.cloud/consider"))
			Expect(service.Spec.TrafficDistribution).To(BeNil())
		})
	})
})
