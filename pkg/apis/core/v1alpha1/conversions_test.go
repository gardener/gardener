// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
)

var _ = Describe("Conversion", func() {
	var scheme *runtime.Scheme

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(SchemeBuilder.AddToScheme(scheme)).ToNot(HaveOccurred())
	})

	Context("seed conversions", func() {
		var blockCIDR = "16.17.18.19/20"

		Describe("#Convert_v1alpha1_Seed_To_core_Seed", func() {
			var (
				out = &core.Seed{}
				in  = &Seed{
					Spec: SeedSpec{
						BlockCIDRs: []string{blockCIDR},
					},
				}
			)

			It("should correctly convert", func() {
				Expect(scheme.Convert(in, out, nil)).To(BeNil())
				Expect(out).To(Equal(&core.Seed{
					Spec: core.SeedSpec{
						Networks: core.SeedNetworks{
							BlockCIDRs: []string{blockCIDR},
						},
					},
				}))
			})
		})

		Describe("#Convert_core_Seed_To_v1alpha1_Seed", func() {
			var (
				out = &Seed{}
				in  = &core.Seed{
					Spec: core.SeedSpec{
						Networks: core.SeedNetworks{
							BlockCIDRs: []string{blockCIDR},
						},
					},
				}
			)

			It("should correctly convert", func() {
				Expect(scheme.Convert(in, out, nil)).To(BeNil())
				Expect(out).To(Equal(&Seed{
					Spec: SeedSpec{
						BlockCIDRs: []string{blockCIDR},
					},
				}))
			})
		})
	})
})
