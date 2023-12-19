// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Migration", func() {
	Describe("#GetResponsibleSeedName", func() {
		It("returns nothing if spec.seedName is not set", func() {
			Expect(GetResponsibleSeedName(nil, nil)).To(BeEmpty())
			Expect(GetResponsibleSeedName(nil, pointer.String("status"))).To(BeEmpty())
		})

		It("returns spec.seedName if status.seedName is not set", func() {
			Expect(GetResponsibleSeedName(pointer.String("spec"), nil)).To(Equal("spec"))
		})

		It("returns status.seedName if the seedNames differ", func() {
			Expect(GetResponsibleSeedName(pointer.String("spec"), pointer.String("status"))).To(Equal("status"))
		})

		It("returns the seedName if both are equal", func() {
			Expect(GetResponsibleSeedName(pointer.String("spec"), pointer.String("spec"))).To(Equal("spec"))
		})
	})
})
