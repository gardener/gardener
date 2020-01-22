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

package operation_test

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/operation"
	operationseed "github.com/gardener/gardener/pkg/operation/seed"
	operationshoot "github.com/gardener/gardener/pkg/operation/shoot"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("operation", func() {
	DescribeTable("#ComputeIngressHost", func(prefix, shootName, projectName, domain string, matcher types.GomegaMatcher) {
		var (
			seed = &gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					DNS: gardencorev1beta1.SeedDNS{
						IngressDomain: domain,
					},
				},
			}
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name: shootName,
				},
			}
			o = &Operation{
				Seed: &operationseed.Seed{
					Info: seed,
				},
				Shoot: &operationshoot.Shoot{
					Info: shoot,
				},
			}
		)

		shoot.Status = gardencorev1beta1.ShootStatus{
			TechnicalID: operationshoot.ComputeTechnicalID(projectName, shoot),
		}

		Expect(o.ComputeIngressHost(prefix)).To(matcher)
	},
		Entry("ingress calculation",
			"t",
			"fooShoot",
			"barProject",
			"ingress.seed.example.com",
			Equal("t-barProject--fooShoot.ingress.seed.example.com"),
		),
	)
})
