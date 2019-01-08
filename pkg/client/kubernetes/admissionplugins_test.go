// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	. "github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ = Describe("kubernetes", func() {
	Describe("#GetAdmissionPluginsForVersion", func() {
		It("should return the list for 1.10 (non-parseable version)", func() {
			expected := []string{"Priority", "NamespaceLifecycle", "LimitRanger", "ServiceAccount", "NodeRestriction", "DefaultStorageClass", "Initializers", "DefaultTolerationSeconds", "ResourceQuota", "StorageObjectInUseProtection", "MutatingAdmissionWebhook", "ValidatingAdmissionWebhook"}

			plugins := GetAdmissionPluginsForVersion("not-1-a-semver-version")

			for _, plugin := range expected {
				Expect(plugins).To(ContainElement(gardenv1beta1.AdmissionPlugin{Name: plugin}))
			}
		})

		It("should return the list for 1.10 (lowest supported version)", func() {
			expected := []string{"Priority", "NamespaceLifecycle", "LimitRanger", "ServiceAccount", "NodeRestriction", "DefaultStorageClass", "Initializers", "DefaultTolerationSeconds", "ResourceQuota", "StorageObjectInUseProtection", "MutatingAdmissionWebhook", "ValidatingAdmissionWebhook"}

			plugins := GetAdmissionPluginsForVersion("1.7.4")

			for _, plugin := range expected {
				Expect(plugins).To(ContainElement(gardenv1beta1.AdmissionPlugin{Name: plugin}))
			}
		})

		It("should return the list for 1.10", func() {
			expected := []string{"Priority", "NamespaceLifecycle", "LimitRanger", "ServiceAccount", "NodeRestriction", "DefaultStorageClass", "Initializers", "DefaultTolerationSeconds", "ResourceQuota", "StorageObjectInUseProtection", "MutatingAdmissionWebhook", "ValidatingAdmissionWebhook"}

			plugins := GetAdmissionPluginsForVersion("1.10.99")

			for _, plugin := range expected {
				Expect(plugins).To(ContainElement(gardenv1beta1.AdmissionPlugin{Name: plugin}))
			}
		})

		It("should return the list for 1.11 or higher", func() {
			expected := []string{"Priority", "NamespaceLifecycle", "LimitRanger", "ServiceAccount", "NodeRestriction", "DefaultStorageClass", "Initializers", "DefaultTolerationSeconds", "ResourceQuota", "StorageObjectInUseProtection", "MutatingAdmissionWebhook", "ValidatingAdmissionWebhook"}

			plugins := GetAdmissionPluginsForVersion("1.11.23")

			for _, plugin := range expected {
				Expect(plugins).To(ContainElement(gardenv1beta1.AdmissionPlugin{Name: plugin}))
			}
		})
	})
})
