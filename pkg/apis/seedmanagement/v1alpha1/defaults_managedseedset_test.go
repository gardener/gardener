// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
