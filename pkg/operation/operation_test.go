// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package operation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/operation"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
)

var _ = Describe("operation", func() {
	DescribeTable("#ComputeIngressHost", func(prefix, shootName, projectName, storedTechnicalID, domain string, matcher gomegatypes.GomegaMatcher) {
		var (
			seed = &gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Ingress: &gardencorev1beta1.Ingress{
						Domain: domain,
					},
				},
			}
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name: shootName,
				},
			}
			o = &Operation{
				Seed:  &seedpkg.Seed{},
				Shoot: &shootpkg.Shoot{},
			}
		)

		shoot.Status = gardencorev1beta1.ShootStatus{
			TechnicalID: storedTechnicalID,
		}
		shoot.Status.TechnicalID = shootpkg.ComputeTechnicalID(projectName, shoot)

		o.Seed.SetInfo(seed)
		o.Shoot.SetInfo(shoot)

		Expect(o.ComputeIngressHost(prefix)).To(matcher)
	},
		Entry("ingress calculation (no stored technical ID)",
			"t",
			"fooShoot",
			"barProject",
			"",
			"ingress.seed.example.com",
			Equal("t-barProject--fooShoot.ingress.seed.example.com"),
		),
		Entry("ingress calculation (historic stored technical ID with a single dash)",
			"t",
			"fooShoot",
			"barProject",
			"shoot-barProject--fooShoot",
			"ingress.seed.example.com",
			Equal("t-barProject--fooShoot.ingress.seed.example.com")),
		Entry("ingress calculation (current stored technical ID with two dashes)",
			"t",
			"fooShoot",
			"barProject",
			"shoot--barProject--fooShoot",
			"ingress.seed.example.com",
			Equal("t-barProject--fooShoot.ingress.seed.example.com")),
	)

	Describe("#ToAdvertisedAddresses", func() {
		var operation *Operation

		BeforeEach(func() {
			operation = &Operation{
				Shoot: &shootpkg.Shoot{},
			}
		})

		It("returns empty list when shoot is nil", func() {
			operation.Shoot = nil

			Expect(operation.ToAdvertisedAddresses()).To(BeNil())
		})
		It("returns external address", func() {
			operation.Shoot.ExternalClusterDomain = pointer.String("foo.bar")

			addresses := operation.ToAdvertisedAddresses()

			Expect(addresses).To(HaveLen(1))
			Expect(addresses).To(ConsistOf(gardencorev1beta1.ShootAdvertisedAddress{
				Name: "external",
				URL:  "https://api.foo.bar",
			}))
		})

		It("returns internal address", func() {
			operation.Shoot.InternalClusterDomain = "baz.foo"

			addresses := operation.ToAdvertisedAddresses()

			Expect(addresses).To(HaveLen(1))
			Expect(addresses).To(ConsistOf(gardencorev1beta1.ShootAdvertisedAddress{
				Name: "internal",
				URL:  "https://api.baz.foo",
			}))
		})

		It("returns unmanaged address", func() {
			operation.APIServerAddress = "bar.foo"

			addresses := operation.ToAdvertisedAddresses()

			Expect(addresses).To(HaveLen(1))
			Expect(addresses).To(ConsistOf(gardencorev1beta1.ShootAdvertisedAddress{
				Name: "unmanaged",
				URL:  "https://bar.foo",
			}))
		})

		It("returns external and internal addresses in correct order", func() {
			operation.Shoot.ExternalClusterDomain = pointer.String("foo.bar")
			operation.Shoot.InternalClusterDomain = "baz.foo"
			operation.APIServerAddress = "bar.foo"

			addresses := operation.ToAdvertisedAddresses()

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
