// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
