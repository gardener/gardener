// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
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
		It("should return the correct list for 1.24", func() {
			expected := []string{"Priority",
				"NamespaceLifecycle",
				"LimitRanger",
				"PodSecurityPolicy",
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

			plugins := GetAdmissionPluginsForVersion("1.24.0")

			for _, plugin := range expected {
				Expect(plugins).To(ContainElement(gardencorev1beta1.AdmissionPlugin{Name: plugin}))
			}
		})

		It("should return the correct list for 1.25 or higher", func() {
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

			plugins := GetAdmissionPluginsForVersion("1.26.0")

			for _, plugin := range expected {
				Expect(plugins).To(ContainElement(gardencorev1beta1.AdmissionPlugin{Name: plugin}))
			}
		})
	})
})
