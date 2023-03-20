// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	componentbaseconfig "k8s.io/component-base/config"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ShootClientMapBuilder", func() {
	var (
		fakeGardenClient       client.Client
		fakeSeedClient         client.Client
		clientConnectionConfig *componentbaseconfig.ClientConnectionConfiguration
	)

	BeforeEach(func() {
		fakeGardenClient = fakeclient.NewClientBuilder().Build()
		fakeSeedClient = fakeclient.NewClientBuilder().Build()
		clientConnectionConfig = &componentbaseconfig.ClientConnectionConfiguration{}
	})

	Describe("#WithGardenClient", func() {
		It("should be correctly set by WithGardenClient", func() {
			builder := NewShootClientMapBuilder().WithGardenClient(fakeGardenClient)
			Expect(builder.gardenClient).To(BeIdenticalTo(fakeGardenClient))
		})
	})

	Describe("#WithSeedClient", func() {
		It("should be correctly set by WithSeedClient", func() {
			builder := NewShootClientMapBuilder().WithSeedClient(fakeSeedClient)
			Expect(builder.seedClient).To(BeIdenticalTo(fakeSeedClient))
		})
	})

	Describe("#WithClientConnectionConfig", func() {
		It("should be correctly set by WithClientConnectionConfig", func() {
			builder := NewShootClientMapBuilder().WithClientConnectionConfig(clientConnectionConfig)
			Expect(builder.clientConnectionConfig).To(BeIdenticalTo(clientConnectionConfig))
		})
	})

	Describe("#Build", func() {
		It("should fail if garden client was not set", func() {
			clientMap, err := NewShootClientMapBuilder().Build(logr.Discard())
			Expect(err).To(MatchError("garden client is required but not set"))
			Expect(clientMap).To(BeNil())
		})

		It("should fail if seed client was not set", func() {
			clientMap, err := NewShootClientMapBuilder().WithGardenClient(fakeGardenClient).Build(logr.Discard())
			Expect(err).To(MatchError("seed client is required but not set"))
			Expect(clientMap).To(BeNil())
		})

		It("should fail if clientConnectionConfig was not set", func() {
			clientMap, err := NewShootClientMapBuilder().WithGardenClient(fakeGardenClient).WithSeedClient(fakeSeedClient).Build(logr.Discard())
			Expect(err).To(MatchError("clientConnectionConfig is required but not set"))
			Expect(clientMap).To(BeNil())
		})

		It("should succeed to build ClientMap", func() {
			clientSet, err := NewShootClientMapBuilder().
				WithGardenClient(fakeGardenClient).
				WithSeedClient(fakeSeedClient).
				WithClientConnectionConfig(clientConnectionConfig).
				Build(logr.Discard())
			Expect(err).NotTo(HaveOccurred())
			Expect(clientSet).NotTo(BeNil())
		})
	})
})
