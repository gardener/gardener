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

		It("should return true if extension classes contain shoot among others", func() {
			Expect(classesHaveClusterObject([]extensionsv1alpha1.ExtensionClass{
				extensionsv1alpha1.ExtensionClassGarden,
				extensionsv1alpha1.ExtensionClassShoot,
			})).To(BeTrue())
		})
	})

	Describe("#isGardenExtensionClass", func() {
		It("should return true for a single garden class", func() {
			Expect(isGardenExtensionClass([]extensionsv1alpha1.ExtensionClass{extensionsv1alpha1.ExtensionClassGarden})).To(BeTrue())
		})

		It("should return false for an empty list", func() {
			Expect(isGardenExtensionClass(nil)).To(BeFalse())
		})

		It("should return false for a single shoot class", func() {
			Expect(isGardenExtensionClass([]extensionsv1alpha1.ExtensionClass{extensionsv1alpha1.ExtensionClassShoot})).To(BeFalse())
		})

		It("should return false for a single seed class", func() {
			Expect(isGardenExtensionClass([]extensionsv1alpha1.ExtensionClass{extensionsv1alpha1.ExtensionClassSeed})).To(BeFalse())
		})

		It("should return false for multiple classes even if garden is included", func() {
			Expect(isGardenExtensionClass([]extensionsv1alpha1.ExtensionClass{
				extensionsv1alpha1.ExtensionClassGarden,
				extensionsv1alpha1.ExtensionClassShoot,
			})).To(BeFalse())
		})
	})

	Describe("#isShootExtensionClass", func() {
		It("should return true for shoot class", func() {
			Expect(isShootExtensionClass(extensionsv1alpha1.ExtensionClassShoot)).To(BeTrue())
		})

		It("should return false for garden class", func() {
			Expect(isShootExtensionClass(extensionsv1alpha1.ExtensionClassGarden)).To(BeFalse())
		})

		It("should return false for seed class", func() {
			Expect(isShootExtensionClass(extensionsv1alpha1.ExtensionClassSeed)).To(BeFalse())
		})

		It("should return false for an empty string", func() {
			Expect(isShootExtensionClass(extensionsv1alpha1.ExtensionClass(""))).To(BeFalse())
		})
	})

	Describe("#getHealthCheckTypes", func() {
		It("should return an empty list for nil input", func() {
			Expect(getHealthCheckTypes(nil)).To(BeEmpty())
		})

		It("should return an empty list for empty input", func() {
			Expect(getHealthCheckTypes([]ConditionTypeToHealthCheck{})).To(BeEmpty())
		})

		It("should return unique condition types", func() {
			healthChecks := []ConditionTypeToHealthCheck{
				{ConditionType: "SystemComponentsHealthy"},
				{ConditionType: "ControlPlaneHealthy"},
				{ConditionType: "SystemComponentsHealthy"},
			}

			result := getHealthCheckTypes(healthChecks)
			Expect(result).To(ConsistOf("SystemComponentsHealthy", "ControlPlaneHealthy"))
		})

		It("should return a single type when all checks share the same condition type", func() {
			healthChecks := []ConditionTypeToHealthCheck{
				{ConditionType: "EveryNodeReady"},
				{ConditionType: "EveryNodeReady"},
			}

			result := getHealthCheckTypes(healthChecks)
			Expect(result).To(ConsistOf("EveryNodeReady"))
		})
	})
})
