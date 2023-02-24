// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
			service := &corev1.Service{}

			ReconcileTopologyAwareRoutingMetadata(service, true)

			Expect(service.Annotations).To(HaveKeyWithValue("service.kubernetes.io/topology-aware-hints", "auto"))
			Expect(service.Labels).To(HaveKeyWithValue("endpoint-slice-hints.resources.gardener.cloud/consider", "true"))
		})

		It("should remove the annotation and label when topology-aware routing is disabled", func() {
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"service.kubernetes.io/topology-aware-hints": "auto"},
					Labels:      map[string]string{"endpoint-slice-hints.resources.gardener.cloud/consider": "true"},
				},
			}

			ReconcileTopologyAwareRoutingMetadata(service, false)

			Expect(service.Annotations).NotTo(HaveKey("service.kubernetes.io/topology-aware-hints"))
			Expect(service.Labels).NotTo(HaveKey("endpoint-slice-hints.resources.gardener.cloud/consider"))
		})
	})
})
