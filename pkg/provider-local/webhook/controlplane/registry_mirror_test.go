// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/provider-local/webhook/controlplane"
)

var _ = Describe("RegistryMirror", func() {
	Describe("HostsTOML", func() {
		It("should return the expected hosts.toml", func() {
			mirror := controlplane.RegistryMirror{UpstreamServer: "http://localhost:5001", MirrorHost: "http://garden.local.gardener.cloud:5001"}

			Expect(mirror.HostsTOML()).To(Equal(`server = "http://localhost:5001"

[host."http://garden.local.gardener.cloud:5001"]
  capabilities = ["pull", "resolve"]
`))
		})
	})
})
