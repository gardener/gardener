// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("TopologyAwareRouting", func() {
	Describe("#ReconcileTopologyAwareRoutingMetadata", func() {
		It("should add the required annotation and label when topology-aware routing is enabled", func() {
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"service.kubernetes.io/topology-aware-hints": "auto",
					},
				},
			}

			ReconcileTopologyAwareRoutingMetadata(service, true)

			Expect(service.Annotations).To(HaveKeyWithValue("service.kubernetes.io/topology-mode", "auto"))
			Expect(service.Annotations).NotTo(HaveKey("service.kubernetes.io/topology-aware-hints"))
			Expect(service.Labels).To(HaveKeyWithValue("endpoint-slice-hints.resources.gardener.cloud/consider", "true"))
		})

		It("should remove the annotations and label when topology-aware routing is disabled", func() {
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"service.kubernetes.io/topology-aware-hints": "auto",
						"service.kubernetes.io/topology-mode":        "auto",
					},
					Labels: map[string]string{"endpoint-slice-hints.resources.gardener.cloud/consider": "true"},
				},
			}

			ReconcileTopologyAwareRoutingMetadata(service, false)

			Expect(service.Annotations).NotTo(HaveKey("service.kubernetes.io/topology-aware-hints"))
			Expect(service.Annotations).NotTo(HaveKey("service.kubernetes.io/topology-mode"))
			Expect(service.Labels).NotTo(HaveKey("endpoint-slice-hints.resources.gardener.cloud/consider"))
		})
	})
})
