// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/api/core"
	"github.com/gardener/gardener/pkg/apis/core"
)

var _ = Describe("Accessor", func() {
	Describe("#Accessor", func() {
		It("Should succeed to create an accessor", func() {
			shoot := &core.Shoot{}
			shootAccessor, err := Accessor(shoot)
			Expect(err).To(Not(HaveOccurred()))
			Expect(shoot).To(Equal(shootAccessor))
		})
	})
})
