// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/client/kubernetes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("kubernetes", func() {
	Describe("#GetAdmissionPluginsForVersion", func() {
		It("should return the list for 1.10 (non-parseable version)", func() {
			expected := []string{"Priority", "NamespaceLifecycle", "LimitRanger", "ServiceAccount", "NodeRestriction", "DefaultStorageClass", "Initializers", "DefaultTolerationSeconds", "ResourceQuota", "StorageObjectInUseProtection", "MutatingAdmissionWebhook", "ValidatingAdmissionWebhook"}

			plugins := GetAdmissionPluginsForVersion("not-1-a-semver-version")

			for _, plugin := range expected {
				Expect(plugins).To(ContainElement(gardencorev1beta1.AdmissionPlugin{Name: plugin}))
			}
		})

		It("should return the list for 1.10 (lowest supported version)", func() {
			expected := []string{"Priority", "NamespaceLifecycle", "LimitRanger", "ServiceAccount", "NodeRestriction", "DefaultStorageClass", "Initializers", "DefaultTolerationSeconds", "ResourceQuota", "StorageObjectInUseProtection", "MutatingAdmissionWebhook", "ValidatingAdmissionWebhook"}

			plugins := GetAdmissionPluginsForVersion("1.7.4")

			for _, plugin := range expected {
				Expect(plugins).To(ContainElement(gardencorev1beta1.AdmissionPlugin{Name: plugin}))
			}
		})

		It("should return the list for 1.10", func() {
			expected := []string{"Priority", "NamespaceLifecycle", "LimitRanger", "ServiceAccount", "NodeRestriction", "DefaultStorageClass", "Initializers", "DefaultTolerationSeconds", "ResourceQuota", "StorageObjectInUseProtection", "MutatingAdmissionWebhook", "ValidatingAdmissionWebhook"}

			plugins := GetAdmissionPluginsForVersion("1.10.99")

			for _, plugin := range expected {
				Expect(plugins).To(ContainElement(gardencorev1beta1.AdmissionPlugin{Name: plugin}))
			}
		})

		It("should return the list for 1.11 or higher", func() {
			expected := []string{"Priority", "NamespaceLifecycle", "LimitRanger", "ServiceAccount", "NodeRestriction", "DefaultStorageClass", "Initializers", "DefaultTolerationSeconds", "ResourceQuota", "StorageObjectInUseProtection", "MutatingAdmissionWebhook", "ValidatingAdmissionWebhook"}

			plugins := GetAdmissionPluginsForVersion("1.11.23")

			for _, plugin := range expected {
				Expect(plugins).To(ContainElement(gardencorev1beta1.AdmissionPlugin{Name: plugin}))
			}
		})

		It("should return the list for 1.14 or higher", func() {
			expected := []string{"Priority", "NamespaceLifecycle", "LimitRanger", "ServiceAccount", "NodeRestriction", "DefaultStorageClass", "DefaultTolerationSeconds", "ResourceQuota", "StorageObjectInUseProtection", "MutatingAdmissionWebhook", "ValidatingAdmissionWebhook"}

			plugins := GetAdmissionPluginsForVersion("1.14.0")

			for _, plugin := range expected {
				Expect(plugins).To(ContainElement(gardencorev1beta1.AdmissionPlugin{Name: plugin}))
			}
		})

		It("should return copy of the default plugins", func() {
			expected := []string{"Priority", "NamespaceLifecycle", "LimitRanger", "ServiceAccount", "NodeRestriction", "DefaultStorageClass", "DefaultTolerationSeconds", "ResourceQuota", "StorageObjectInUseProtection", "MutatingAdmissionWebhook", "ValidatingAdmissionWebhook"}

			plugins := GetAdmissionPluginsForVersion("1.14.0")
			plugins2 := GetAdmissionPluginsForVersion("1.14.0")
			plugins[0].Name = "MissingPlugin"

			for _, plugin := range expected {
				Expect(plugins2).To(ContainElement(gardencorev1beta1.AdmissionPlugin{Name: plugin}))
			}
		})
	})
})
