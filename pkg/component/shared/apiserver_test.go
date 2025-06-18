// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/schema"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/component/shared"
)

var _ = Describe("APIServer", func() {
	Describe("#GetResourcesForEncryptionFromConfig", func() {
		It("should return nil when encryptionConfig is nil", func() {
			Expect(GetResourcesForEncryptionFromConfig(nil)).To(BeNil())
		})

		It("should return the correct list of resources when encryptionConfig is not nil", func() {
			encryptionConfig := &gardencorev1beta1.EncryptionConfig{
				Resources: []string{"deployments.apps", "fancyresource.customoperator.io", "configmaps", "services.", "daemonsets.apps"},
			}

			Expect(GetResourcesForEncryptionFromConfig(encryptionConfig)).To(ConsistOf(
				schema.GroupResource{Resource: "deployments", Group: "apps"},
				schema.GroupResource{Resource: "fancyresource", Group: "customoperator.io"},
				schema.GroupResource{Resource: "configmaps", Group: ""},
				schema.GroupResource{Resource: "services", Group: ""},
				schema.GroupResource{Resource: "daemonsets", Group: "apps"},
			))
		})
	})

	Describe("#StringifyGroupResources", func() {
		It("should return nil when resources are nil", func() {
			Expect(StringifyGroupResources(nil)).To(BeNil())
		})

		It("should return the correct list of resources when resources are not nil", func() {
			resources := []schema.GroupResource{
				schema.GroupResource{Resource: "deployments", Group: "apps"},
				schema.GroupResource{Resource: "fancyresource", Group: "customoperator.io"},
				schema.GroupResource{Resource: "configmaps", Group: ""},
				schema.GroupResource{Resource: "services", Group: ""},
				schema.GroupResource{Resource: "daemonsets", Group: "apps"},
			}

			Expect(StringifyGroupResources(resources)).To(ConsistOf(
				"deployments.apps",
				"fancyresource.customoperator.io",
				"configmaps",
				"services",
				"daemonsets.apps",
			))
		})
	})

	Describe("#NormalizeResources", func() {
		It("should return nil when encryptionConfig is nil", func() {
			Expect(NormalizeResources(nil)).To(BeNil())
		})

		It("should return the correct list of resources when encryptionConfig is not nil", func() {
			resources := []string{"deployments.apps", "fancyresource.customoperator.io", "endpoints.", "configmaps", "services."}

			Expect(NormalizeResources(resources)).To(ConsistOf(
				"deployments.apps",
				"fancyresource.customoperator.io",
				"configmaps",
				"services",
				"endpoints",
			))
		})
	})
})
