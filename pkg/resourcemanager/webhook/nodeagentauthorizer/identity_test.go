// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagentauthorizer_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apiserver/pkg/authentication/user"

	. "github.com/gardener/gardener/pkg/resourcemanager/webhook/nodeagentauthorizer"
)

var _ = Describe("Identity", func() {
	var userInfo user.DefaultInfo

	It("should return the correct gardener-node-agent user", func() {
		userInfo.Name = "gardener.cloud:node-agent:machine:foo-user"
		userName, gardenerUser := GetNodeAgentIdentity(&userInfo)
		Expect(gardenerUser).To(BeTrue())
		Expect(userName).To(Equal("foo-user"))
	})

	It("should return false for a non gardener-node-agent user", func() {
		userInfo.Name = "bar-user"
		userName, gardenerUser := GetNodeAgentIdentity(&userInfo)
		Expect(gardenerUser).To(BeFalse())
		Expect(userName).To(BeEmpty())
	})
})
