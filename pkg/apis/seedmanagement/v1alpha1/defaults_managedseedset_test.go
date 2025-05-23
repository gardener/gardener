// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
)

var _ = Describe("Defaults", func() {
	var obj *ManagedSeedSet

	BeforeEach(func() {
		obj = &ManagedSeedSet{}
	})

	Describe("ManagedSeedSet defaulting", func() {
		It("should default replicas to 1 and revisionHistoryLimit to 10", func() {
			SetObjectDefaults_ManagedSeedSet(obj)

			Expect(obj.Spec.Replicas).To(Equal(ptr.To[int32](1)))
			Expect(obj.Spec.UpdateStrategy).NotTo(BeNil())
			Expect(obj.Spec.RevisionHistoryLimit).To(Equal(ptr.To[int32](10)))
		})

		It("should not overwrite the already set values for ManagedSeedSet spec", func() {
			obj.Spec = ManagedSeedSetSpec{
				Replicas:             ptr.To[int32](5),
				RevisionHistoryLimit: ptr.To[int32](15),
			}
			SetObjectDefaults_ManagedSeedSet(obj)

			Expect(obj.Spec.Replicas).To(Equal(ptr.To[int32](5)))
			Expect(obj.Spec.UpdateStrategy).NotTo(BeNil())
			Expect(obj.Spec.RevisionHistoryLimit).To(Equal(ptr.To[int32](15)))
		})
	})

	Describe("UpdateStrategy defaulting", func() {
		It("should default type to RollingUpdate", func() {
			obj.Spec.UpdateStrategy = &UpdateStrategy{}
			SetObjectDefaults_ManagedSeedSet(obj)

			Expect(obj.Spec.UpdateStrategy).To(Equal(&UpdateStrategy{
				Type: ptr.To(RollingUpdateStrategyType),
			}))
		})

		It("should not overwrite already set values for UpdateStrategy", func() {
			obj.Spec.UpdateStrategy = &UpdateStrategy{
				Type: ptr.To(UpdateStrategyType("foo")),
			}
			SetObjectDefaults_ManagedSeedSet(obj)

			Expect(obj.Spec.UpdateStrategy).To(Equal(&UpdateStrategy{
				Type: ptr.To(UpdateStrategyType("foo")),
			}))
		})
	})

	Describe("RollingUpdateStrategy defaulting", func() {
		It("should default partition to 0", func() {
			obj.Spec.UpdateStrategy = &UpdateStrategy{
				RollingUpdate: &RollingUpdateStrategy{},
			}
			SetObjectDefaults_ManagedSeedSet(obj)

			Expect(obj.Spec.UpdateStrategy.RollingUpdate).To(Equal(&RollingUpdateStrategy{
				Partition: ptr.To[int32](0),
			}))
		})

		It("should not overwrote the already set values for RollingUpdateStrategy", func() {
			obj.Spec.UpdateStrategy = &UpdateStrategy{
				RollingUpdate: &RollingUpdateStrategy{
					Partition: ptr.To[int32](1),
				},
			}
			SetObjectDefaults_ManagedSeedSet(obj)

			Expect(obj.Spec.UpdateStrategy.RollingUpdate).To(Equal(&RollingUpdateStrategy{
				Partition: ptr.To[int32](1),
			}))
		})
	})
})
