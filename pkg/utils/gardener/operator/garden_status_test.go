// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package operator_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/component-base/version"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/gardener/operator"
)

var _ = Describe("GardenStatus", func() {
	Describe("#IsGardenSuccessfullyReconciled", func() {
		var garden *operatorv1alpha1.Garden

		BeforeEach(func() {
			garden = &operatorv1alpha1.Garden{}
		})

		It("should return false if last operation is not available", func() {
			Expect(IsGardenSuccessfullyReconciled(garden)).Should(BeFalse())
		})

		It("should return false if last operation is not reconcile", func() {
			garden.Status.LastOperation = &gardencorev1beta1.LastOperation{
				Type:     "Delete",
				State:    "Succeeded",
				Progress: 100,
			}

			Expect(IsGardenSuccessfullyReconciled(garden)).Should(BeFalse())
		})

		It("should return false if last operation is not succeeded", func() {
			garden.Status.LastOperation = &gardencorev1beta1.LastOperation{
				Type:     "Reconcile",
				State:    "Failed",
				Progress: 100,
			}

			Expect(IsGardenSuccessfullyReconciled(garden)).Should(BeFalse())
		})

		It("should return false if last operation is not finished", func() {
			garden.Status.LastOperation = &gardencorev1beta1.LastOperation{
				Type:     "Reconcile",
				State:    "Succeeded",
				Progress: 99,
			}

			Expect(IsGardenSuccessfullyReconciled(garden)).Should(BeFalse())
		})

		It("should return true if last operation is finished and succeeded", func() {
			garden.Status.LastOperation = &gardencorev1beta1.LastOperation{
				Type:     "Reconcile",
				State:    "Succeeded",
				Progress: 100,
			}

			Expect(IsGardenSuccessfullyReconciled(garden)).Should(BeTrue())
		})
	})

	Describe("#IsGardenUpToDate", func() {
		var garden *operatorv1alpha1.Garden

		BeforeEach(func() {
			garden = &operatorv1alpha1.Garden{
				Status: operatorv1alpha1.GardenStatus{
					Gardener: &gardencorev1beta1.Gardener{},
				},
			}
		})

		It("should return false if the garden version does not match the operator version", func() {
			garden.Status.Gardener.Version = "v0.0.0-outdated"
			Expect(IsGardenUpToDate(garden)).Should(BeFalse())
		})

		It("should return true if the garden version matches the operator version", func() {
			garden.Status.Gardener.Version = version.Get().GitVersion
			Expect(IsGardenUpToDate(garden)).Should(BeTrue())
		})
	})
})
