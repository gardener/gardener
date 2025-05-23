// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package lease_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Reconciler", func() {
	Describe("#ObjectName", func() {
		It("should return the expected name", func() {
			Expect(gardenerutils.NodeAgentLeaseName("foo")).To(Equal("gardener-node-agent-foo"))
		})
	})
})
