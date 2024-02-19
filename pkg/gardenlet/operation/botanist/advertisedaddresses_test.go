// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("AdvertisedAddresses", func() {
	var (
		botanist *Botanist
	)

	BeforeEach(func() {
		botanist = &Botanist{Operation: &operation.Operation{}}
		botanist.Shoot = &shootpkg.Shoot{}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
	})

	Describe("#ToAdvertisedAddresses", func() {
		It("returns empty list when shoot is nil", func() {
			botanist.Shoot = nil

			Expect(botanist.ToAdvertisedAddresses()).To(BeNil())
		})

		It("returns external address", func() {
			botanist.Shoot.ExternalClusterDomain = ptr.To("foo.bar")

			addresses := botanist.ToAdvertisedAddresses()

			Expect(addresses).To(HaveLen(1))
			Expect(addresses).To(ConsistOf(gardencorev1beta1.ShootAdvertisedAddress{
				Name: "external",
				URL:  "https://api.foo.bar",
			}))
		})

		It("returns internal address", func() {
			botanist.Shoot.InternalClusterDomain = "baz.foo"

			addresses := botanist.ToAdvertisedAddresses()

			Expect(addresses).To(HaveLen(1))
			Expect(addresses).To(ConsistOf(gardencorev1beta1.ShootAdvertisedAddress{
				Name: "internal",
				URL:  "https://api.baz.foo",
			}))
		})

		It("returns unmanaged address", func() {
			botanist.APIServerAddress = "bar.foo"

			addresses := botanist.ToAdvertisedAddresses()

			Expect(addresses).To(HaveLen(1))
			Expect(addresses).To(ConsistOf(gardencorev1beta1.ShootAdvertisedAddress{
				Name: "unmanaged",
				URL:  "https://bar.foo",
			}))
		})

		It("returns external and internal addresses in correct order", func() {
			botanist.Shoot.ExternalClusterDomain = ptr.To("foo.bar")
			botanist.Shoot.InternalClusterDomain = "baz.foo"
			botanist.APIServerAddress = "bar.foo"

			addresses := botanist.ToAdvertisedAddresses()

			Expect(addresses).To(Equal([]gardencorev1beta1.ShootAdvertisedAddress{
				{
					Name: "external",
					URL:  "https://api.foo.bar",
				}, {
					Name: "internal",
					URL:  "https://api.baz.foo",
				},
			}))
		})
	})
})
