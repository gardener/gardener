// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ShootClientMapBuilder", func() {
	var (
		fakeGardenClient       client.Client
		fakeSeedClient         client.Client
		clientConnectionConfig *componentbaseconfigv1alpha1.ClientConnectionConfiguration
	)

	BeforeEach(func() {
		fakeGardenClient = fakeclient.NewClientBuilder().Build()
		fakeSeedClient = fakeclient.NewClientBuilder().Build()
		clientConnectionConfig = &componentbaseconfigv1alpha1.ClientConnectionConfiguration{}
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
