// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mediumtouch

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("gardenadm medium-touch scenario tests", Label("gardenadm", "medium-touch"), func() {
	BeforeEach(OncePerOrdered, func(SpecContext) {
		PrepareBinary()
	}, NodeTimeout(time.Minute))

	Describe("Prepare infrastructure and machines", Ordered, func() {
		It("should bootstrap the machine pods", func(SpecContext) {
			Eventually(RunAndWait("bootstrap")).Should(gbytes.Say("not implemented as well"))
		}, SpecTimeout(time.Minute))
	})
})
