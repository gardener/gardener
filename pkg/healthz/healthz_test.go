// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthz_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/healthz"
)

var _ = Describe("Healthz", func() {
	Describe("#HandlerFunc", func() {
		var (
			ctx     = context.TODO()
			healthz Manager
		)

		BeforeEach(func() {
			healthz = NewDefaultHealthz()
			Expect(healthz.Start(ctx)).To(Succeed())
		})

		It("should return a function that returns nil when the health check passes", func() {
			healthz.Set(true)
			Expect(CheckerFunc(healthz)(nil)).To(Succeed())
		})

		It("should return a function that returns an error when the health check does not pass", func() {
			healthz.Set(false)
			Expect(CheckerFunc(healthz)(nil)).To(MatchError(ContainSubstring("current health status is 'unhealthy'")))
		})
	})
})
