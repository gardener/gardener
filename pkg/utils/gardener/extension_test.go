// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Extension", func() {
	Describe("#ExtensionAdmissionRuntimeManagedResourceName", func() {
		It("should return the expected managed resource name", func() {
			Expect(ExtensionAdmissionRuntimeManagedResourceName("provider-test")).To(Equal("extension-admission-runtime-provider-test"))
		})
	})

	Describe("#ExtensionAdmissionVirtualManagedResourceName", func() {
		It("should return the expected managed resource name", func() {
			Expect(ExtensionAdmissionVirtualManagedResourceName("provider-test")).To(Equal("extension-admission-virtual-provider-test"))
		})
	})

	Describe("#ExtensionRuntimeManagedResourceName", func() {
		It("should return the expected managed resource name", func() {
			Expect(ExtensionRuntimeManagedResourceName("provider-test")).To(Equal("extension-provider-test-garden"))
		})
	})

	DescribeTable("#ExtensionForManagedResourceName", func(managedResourceName string, expectedExtensionName string, expectedIsExtension bool) {
		extensionName, isExtension := ExtensionForManagedResourceName(managedResourceName)
		Expect(extensionName).To(Equal(expectedExtensionName))
		Expect(isExtension).To(Equal(expectedIsExtension))
	},

		Entry("it should recognize a managed resource of an extension", "extension-foobar-garden", "foobar", true),
		Entry("it should recognize a managed resource of an extension admission for runtime cluster", "extension-admission-runtime-foobar", "foobar", true),
		Entry("it should recognize a managed resource of an extension admission for virtual cluster", "extension-admission-virtual-foobar", "foobar", true),
		Entry("it should not recognize a random managed resource as an extension", "foobar", "", false),
		Entry("it should not recognize a managed resource with a matching prefix only as an extension", "extension-foobar", "", false),
		Entry("it should not recognize a managed resource with a matching suffix only as an extension", "foobar-garden", "", false),
	)

	Describe("#ExtensionRuntimeNamespaceName", func() {
		It("should return the expected namespace name", func() {
			Expect(ExtensionRuntimeNamespaceName("provider-test")).To(Equal("runtime-extension-provider-test"))
		})
	})

	Describe("#IsControllerInstallationInVirtualRequired", func() {
		It("should return true if the extension requires a controller installation in the virtual cluster", func() {
			Expect(IsControllerInstallationInVirtualRequired(&operatorv1alpha1.Extension{
				Status: operatorv1alpha1.ExtensionStatus{
					Conditions: []gardencorev1beta1.Condition{{Type: "RequiredVirtual", Status: "True"}},
				},
			})).To(BeTrue())
		})
	})

	Describe("#IsExtensionInRuntimeRequired", func() {
		It("should return true if the extension requires a deployment in the runtime cluster", func() {
			Expect(IsExtensionInRuntimeRequired(&operatorv1alpha1.Extension{
				Status: operatorv1alpha1.ExtensionStatus{
					Conditions: []gardencorev1beta1.Condition{{Type: "RequiredRuntime", Status: "True"}},
				},
			})).To(BeTrue())
		})
	})
})
