// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	. "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
)

var _ = Describe("Defaults", func() {
	Describe("#SetDefaults_ManagedSeedSet", func() {
		var obj *ManagedSeedSet

		BeforeEach(func() {
			obj = &ManagedSeedSet{}
		})

		It("should default replicas to 1 and revisionHistoryLimit to 10", func() {
			SetDefaults_ManagedSeedSet(obj)

			Expect(obj).To(Equal(&ManagedSeedSet{
				Spec: ManagedSeedSetSpec{
					Replicas:             pointer.Int32(1),
					UpdateStrategy:       &UpdateStrategy{},
					RevisionHistoryLimit: pointer.Int32(10),
				},
			}))
		})
	})

	Describe("#SetDefaults_UpdateStrategy", func() {
		var obj *UpdateStrategy

		BeforeEach(func() {
			obj = &UpdateStrategy{}
		})

		It("should default type to RollingUpdate", func() {
			SetDefaults_UpdateStrategy(obj)

			Expect(obj).To(Equal(&UpdateStrategy{
				Type: updateStrategyTypePtr(RollingUpdateStrategyType),
			}))
		})
	})

	Describe("#SetDefaults_RollingUpdateStrategy", func() {
		var obj *RollingUpdateStrategy

		BeforeEach(func() {
			obj = &RollingUpdateStrategy{}
		})

		It("should default partition to 0", func() {
			SetDefaults_RollingUpdateStrategy(obj)

			Expect(obj).To(Equal(&RollingUpdateStrategy{
				Partition: pointer.Int32(0),
			}))
		})
	})
})

func updateStrategyTypePtr(v UpdateStrategyType) *UpdateStrategyType {
	return &v
}
