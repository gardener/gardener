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

var _ = Describe("GardenClientMapBuilder", func() {
	var (
		fakeRuntimeClient      client.Client
		clientConnectionConfig *componentbaseconfigv1alpha1.ClientConnectionConfiguration
	)

	BeforeEach(func() {
		fakeRuntimeClient = fakeclient.NewClientBuilder().Build()
		clientConnectionConfig = &componentbaseconfigv1alpha1.ClientConnectionConfiguration{}
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
