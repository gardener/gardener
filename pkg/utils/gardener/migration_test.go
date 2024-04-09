// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardener_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Migration", func() {
	Describe("#GetResponsibleSeedName", func() {
		It("returns nothing if spec.seedName is not set", func() {
			Expect(GetResponsibleSeedName(nil, nil)).To(BeEmpty())
			Expect(GetResponsibleSeedName(nil, ptr.To("status"))).To(BeEmpty())
		})

		It("returns spec.seedName if status.seedName is not set", func() {
			Expect(GetResponsibleSeedName(ptr.To("spec"), nil)).To(Equal("spec"))
		})

		It("returns status.seedName if the seedNames differ", func() {
			Expect(GetResponsibleSeedName(ptr.To("spec"), ptr.To("status"))).To(Equal("status"))
		})

		It("returns the seedName if both are equal", func() {
			Expect(GetResponsibleSeedName(ptr.To("spec"), ptr.To("spec"))).To(Equal("spec"))
		})
	})
})
