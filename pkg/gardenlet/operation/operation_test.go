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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/gardenlet/operation"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
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
		shoot.Status.TechnicalID = gardenerutils.ComputeTechnicalID(projectName, shoot)

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
})
