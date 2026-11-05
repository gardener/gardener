// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	. "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("ExtensionProfile defaulting", func() {
	var obj *ExtensionProfile

	BeforeEach(func() {
		obj = &ExtensionProfile{
			Spec: ExtensionProfileSpec{
				UpdateStrategy: ExtensionUpdateStrategy{
					Supported: []UpdateStrategy{UpdateStrategyPatch},
				},
			},
		}
	})

	Describe("#SetDefaults_ExtensionProfile", func() {
		It("should default the profile-level update strategy to patch when Default is nil", func() {
			obj.Spec.UpdateStrategy.Default = nil

			SetDefaults_ExtensionProfile(obj)

			Expect(obj.Spec.UpdateStrategy.Default).To(PointTo(Equal(UpdateStrategyPatch)))
		})

		It("should append the defaulted patch strategy to Supported if absent", func() {
			obj.Spec.UpdateStrategy.Default = nil
			obj.Spec.UpdateStrategy.Supported = []UpdateStrategy{UpdateStrategyMinor}

			SetDefaults_ExtensionProfile(obj)

			Expect(obj.Spec.UpdateStrategy.Default).To(PointTo(Equal(UpdateStrategyPatch)))
			Expect(obj.Spec.UpdateStrategy.Supported).To(ConsistOf(UpdateStrategyMinor, UpdateStrategyPatch))
		})

		It("should not overwrite an already-set Default and append it to Supported if absent", func() {
			obj.Spec.UpdateStrategy.Default = new(UpdateStrategyMinor)
			obj.Spec.UpdateStrategy.Supported = []UpdateStrategy{UpdateStrategyPatch}

			SetDefaults_ExtensionProfile(obj)

			Expect(obj.Spec.UpdateStrategy.Default).To(PointTo(Equal(UpdateStrategyMinor)))
			Expect(obj.Spec.UpdateStrategy.Supported).To(ConsistOf(UpdateStrategyPatch, UpdateStrategyMinor))
		})

		It("should not append a duplicate when Default is already contained in Supported", func() {
			obj.Spec.UpdateStrategy.Default = new(UpdateStrategyPatch)
			obj.Spec.UpdateStrategy.Supported = []UpdateStrategy{UpdateStrategyPatch, UpdateStrategyMinor}

			SetDefaults_ExtensionProfile(obj)

			Expect(obj.Spec.UpdateStrategy.Default).To(PointTo(Equal(UpdateStrategyPatch)))
			Expect(obj.Spec.UpdateStrategy.Supported).To(ConsistOf(UpdateStrategyPatch, UpdateStrategyMinor))
		})

		It("should apply the same defaulting to a component with a non-nil update strategy", func() {
			obj.Spec.Components = []ExtensionProfileComponent{{
				Name: "traefik",
				UpdateStrategy: &ExtensionUpdateStrategy{
					Supported: []UpdateStrategy{UpdateStrategyMinor},
				},
			}}

			SetDefaults_ExtensionProfile(obj)

			Expect(obj.Spec.Components[0].UpdateStrategy.Default).To(PointTo(Equal(UpdateStrategyPatch)))
			Expect(obj.Spec.Components[0].UpdateStrategy.Supported).To(ConsistOf(UpdateStrategyMinor, UpdateStrategyPatch))
		})

		It("should leave a component with a nil update strategy untouched", func() {
			obj.Spec.Components = []ExtensionProfileComponent{{
				Name: "traefik",
			}}

			SetDefaults_ExtensionProfile(obj)

			Expect(obj.Spec.Components[0].UpdateStrategy).To(BeNil())
		})
	})
})
