// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"github.com/gardener/gardener/pkg/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

var _ = Describe("GardenClientMapBuilder", func() {

	var (
		fakeLogger logrus.FieldLogger
		restConfig *rest.Config
	)

	BeforeEach(func() {
		fakeLogger = logger.NewNopLogger()
		restConfig = &rest.Config{}
	})

	Context("#logger", func() {
		It("should be set correctly by WithLogger", func() {
			builder := NewGardenClientMapBuilder().WithLogger(fakeLogger)
			Expect(builder.logger).To(BeIdenticalTo(fakeLogger))
		})
	})

	Context("#restConfig", func() {
		It("should be set correctly by WithRESTConfig", func() {
			builder := NewGardenClientMapBuilder().WithRESTConfig(restConfig)
			Expect(builder.restConfig).To(BeIdenticalTo(restConfig))
		})
	})

	Context("#Build", func() {
		It("should fail if logger was not set", func() {
			clientMap, err := NewGardenClientMapBuilder().Build()
			Expect(err).To(MatchError("logger is required but not set"))
			Expect(clientMap).To(BeNil())
		})

		It("should fail if restConfig was not set", func() {
			clientMap, err := NewGardenClientMapBuilder().WithLogger(fakeLogger).Build()
			Expect(err).To(MatchError("restConfig is required but not set"))
			Expect(clientMap).To(BeNil())
		})

		It("should succeed to build ClientMap", func() {
			clientSet, err := NewGardenClientMapBuilder().
				WithRESTConfig(restConfig).
				WithLogger(fakeLogger).
				Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(clientSet).NotTo(BeNil())
		})
	})

})
