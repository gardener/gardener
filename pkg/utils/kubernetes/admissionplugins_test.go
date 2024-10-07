// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("kubernetes", func() {
	Describe("#GetAdmissionPluginsForVersion", func() {
		It("should return the correct list for 1.27", func() {
			expected := []string{"Priority",
				"NamespaceLifecycle",
				"LimitRanger",
				"PodSecurity",
				"ServiceAccount",
				"NodeRestriction",
				"DefaultStorageClass",
				"DefaultTolerationSeconds",
				"ResourceQuota",
				"StorageObjectInUseProtection",
				"MutatingAdmissionWebhook",
				"ValidatingAdmissionWebhook",
			}

			plugins := GetAdmissionPluginsForVersion("1.27.0")

			for _, plugin := range expected {
				Expect(plugins).To(ContainElement(gardencorev1beta1.AdmissionPlugin{Name: plugin}))
			}
		})

		It("should return the correct list for > 1.27", func() {
			expected := []string{"Priority",
				"NamespaceLifecycle",
				"LimitRanger",
				"PodSecurity",
				"ServiceAccount",
				"NodeRestriction",
				"DefaultStorageClass",
				"DefaultTolerationSeconds",
				"ResourceQuota",
				"StorageObjectInUseProtection",
				"MutatingAdmissionWebhook",
				"ValidatingAdmissionWebhook",
			}

			plugins := GetAdmissionPluginsForVersion("1.28.0")

			for _, plugin := range expected {
				Expect(plugins).To(ContainElement(gardencorev1beta1.AdmissionPlugin{Name: plugin}))
			}
		})
	})
})
