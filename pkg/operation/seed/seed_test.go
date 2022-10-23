// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/operation/seed"
)

var _ = Describe("seed", func() {
	Describe("#GetValidVolumeSize", func() {
		It("should return the size because no minimum size was set", func() {
			var (
				size = "20Gi"
				seed = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Volume: nil,
				},
			})

			Expect(seed.GetValidVolumeSize(size)).To(Equal(size))
		})

		It("should return the minimum size because the given value is smaller", func() {
			var (
				size                = "20Gi"
				minimumSize         = "25Gi"
				minimumSizeQuantity = resource.MustParse(minimumSize)
				seed                = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Volume: &gardencorev1beta1.SeedVolume{
						MinimumSize: &minimumSizeQuantity,
					},
				},
			})

			Expect(seed.GetValidVolumeSize(size)).To(Equal(minimumSize))
		})

		It("should return the given value size because the minimum size is smaller", func() {
			var (
				size                = "30Gi"
				minimumSize         = "25Gi"
				minimumSizeQuantity = resource.MustParse(minimumSize)
				seed                = &Seed{}
			)
			seed.SetInfo(&gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Volume: &gardencorev1beta1.SeedVolume{
						MinimumSize: &minimumSizeQuantity,
					},
				},
			})

			Expect(seed.GetValidVolumeSize(size)).To(Equal(size))
		})
	})
})
