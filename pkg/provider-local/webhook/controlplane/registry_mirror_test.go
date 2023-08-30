// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
