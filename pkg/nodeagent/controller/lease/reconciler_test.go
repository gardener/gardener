// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package lease_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/nodeagent/controller/lease"
)

var _ = Describe("Reconciler", func() {
	Describe("#ObjectName", func() {
		It("should return the expected name", func() {
			Expect(ObjectName("foo")).To(Equal("gardener-node-agent-foo"))
		})
	})
})
