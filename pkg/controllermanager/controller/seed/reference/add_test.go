// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reference_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/seed/reference"
)

var _ = Describe("Add", func() {
	Describe("#Predicate", func() {
		var seed *gardencorev1beta1.Seed

		BeforeEach(func() {
			seed = &gardencorev1beta1.Seed{}
		})

		It("should return false because new object is no seed", func() {
			Expect(Predicate(nil, nil)).To(BeFalse())
		})

		It("should return false because old object is no seed", func() {
			Expect(Predicate(nil, seed)).To(BeFalse())
		})

		It("should return false because there is no ref change", func() {
			Expect(Predicate(seed, seed)).To(BeFalse())
		})

		It("should return true because the resources field changed", func() {
			oldSeed := seed.DeepCopy()
			seed.Spec.Resources = []gardencorev1beta1.NamedResourceReference{{
				Name: "resource-1",
				ResourceRef: autoscalingv1.CrossVersionObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Name:       "test",
				},
			}}
			Expect(Predicate(oldSeed, seed)).To(BeTrue())
		})
	})
})
