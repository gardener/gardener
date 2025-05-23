// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operator_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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
})
