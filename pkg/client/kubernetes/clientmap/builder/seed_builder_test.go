// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	baseconfig "k8s.io/component-base/config"
)

var _ = Describe("SeedClientMapBuilder", func() {
	var (
		clientConnectionConfig *baseconfig.ClientConnectionConfiguration
	)

	BeforeEach(func() {
		clientConnectionConfig = &baseconfig.ClientConnectionConfiguration{}
	})

	Context("#clientConnectionConfig", func() {
		It("should be correctly set by WithClientConnectionConfig", func() {
			builder := NewSeedClientMapBuilder().WithClientConnectionConfig(clientConnectionConfig)
			Expect(builder.clientConnectionConfig).To(BeIdenticalTo(clientConnectionConfig))
		})
	})

	Context("#Build", func() {
		It("should fail if clientConnectionConfig was not set", func() {
			clientMap, err := NewSeedClientMapBuilder().Build()
			Expect(err).To(MatchError("clientConnectionConfig is required but not set"))
			Expect(clientMap).To(BeNil())
		})

		It("should succeed to build ClientMap", func() {
			clientSet, err := NewSeedClientMapBuilder().
				WithClientConnectionConfig(clientConnectionConfig).
				Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(clientSet).NotTo(BeNil())
		})
	})

})
