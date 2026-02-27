// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthcheck

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

var _ = Describe("controller", func() {
	Describe("#classesHaveClusterObject", func() {
		It("should return true if extension classes contain shoot", func() {
			Expect(classesHaveClusterObject([]extensionsv1alpha1.ExtensionClass{extensionsv1alpha1.ExtensionClassShoot})).To(BeTrue())
		})

		It("should return true if extension classes is empty", func() {
			Expect(classesHaveClusterObject(nil)).To(BeTrue())
		})

		It("should return false if extension classes are seed and garden", func() {
			Expect(classesHaveClusterObject([]extensionsv1alpha1.ExtensionClass{
				extensionsv1alpha1.ExtensionClassGarden,
				extensionsv1alpha1.ExtensionClassSeed,
			})).To(BeFalse())
		})
	})
})
