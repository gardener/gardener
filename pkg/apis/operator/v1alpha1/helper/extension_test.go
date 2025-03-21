// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/operator/v1alpha1/helper"
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

	Describe("#ExtensionRuntimeNamespaceName", func() {
		It("should return the expected namespace name", func() {
			Expect(ExtensionRuntimeNamespaceName("provider-test")).To(Equal("runtime-extension-provider-test"))
		})
	})

	Describe("#IsDeploymentInRuntimeRequired", func() {
		It("should return true if the extension requires a deployment in the runtime cluster", func() {
			Expect(IsDeploymentInRuntimeRequired(&operatorv1alpha1.Extension{
				Status: operatorv1alpha1.ExtensionStatus{
					Conditions: []gardencorev1beta1.Condition{{Type: "RequiredRuntime", Status: "True"}},
				},
			})).To(BeTrue())
		})
	})
})
