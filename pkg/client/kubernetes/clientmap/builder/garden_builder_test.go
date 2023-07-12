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

package builder

import (
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	componentbaseconfig "k8s.io/component-base/config"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("GardenClientMapBuilder", func() {
	var (
		fakeRuntimeClient      client.Client
		clientConnectionConfig *componentbaseconfig.ClientConnectionConfiguration
	)

	BeforeEach(func() {
		fakeRuntimeClient = fakeclient.NewClientBuilder().Build()
		clientConnectionConfig = &componentbaseconfig.ClientConnectionConfiguration{}
	})

	Describe("#WithRuntimeClient", func() {
		It("should be correctly set by WithRuntimeClient", func() {
			builder := NewGardenClientMapBuilder().WithRuntimeClient(fakeRuntimeClient)
			Expect(builder.runtimeClient).To(BeIdenticalTo(fakeRuntimeClient))
		})
	})

	Describe("#WithClientConnectionConfig", func() {
		It("should be correctly set by WithClientConnectionConfig", func() {
			builder := NewGardenClientMapBuilder().WithClientConnectionConfig(clientConnectionConfig)
			Expect(builder.clientConnectionConfig).To(BeIdenticalTo(clientConnectionConfig))
		})
	})

	Describe("#WithGardenNamespace", func() {
		It("should be correctly set by WithGardenNamespace", func() {
			builder := NewGardenClientMapBuilder().WithGardenNamespace("foo")
			Expect(builder.gardenNamespace).To(Equal("foo"))
		})
	})

	Describe("#!WithGardenNamespace", func() {
		It("should be correctly set with WithGardenNamespace", func() {
			builder := NewGardenClientMapBuilder()
			Expect(builder.gardenNamespace).To(Equal(""))
		})
	})

	Describe("#Build", func() {
		It("should fail if runtime client was not set", func() {
			clientMap, err := NewGardenClientMapBuilder().Build(logr.Discard())
			Expect(err).To(MatchError("runtime client is required but not set"))
			Expect(clientMap).To(BeNil())
		})

		It("should fail if clientConnectionConfig was not set", func() {
			clientMap, err := NewGardenClientMapBuilder().WithRuntimeClient(fakeRuntimeClient).Build(logr.Discard())
			Expect(err).To(MatchError("clientConnectionConfig is required but not set"))
			Expect(clientMap).To(BeNil())
		})

		It("should succeed to build ClientMap", func() {
			clientSet, err := NewGardenClientMapBuilder().
				WithRuntimeClient(fakeRuntimeClient).
				WithClientConnectionConfig(clientConnectionConfig).
				Build(logr.Discard())
			Expect(err).NotTo(HaveOccurred())
			Expect(clientSet).NotTo(BeNil())
		})

		It("should succeed to build ClientMap if gardenNamespace is set", func() {
			clientSet, err := NewGardenClientMapBuilder().
				WithRuntimeClient(fakeRuntimeClient).
				WithClientConnectionConfig(clientConnectionConfig).
				WithGardenNamespace("foo").
				Build(logr.Discard())
			Expect(err).NotTo(HaveOccurred())
			Expect(clientSet).NotTo(BeNil())
		})
	})
})
