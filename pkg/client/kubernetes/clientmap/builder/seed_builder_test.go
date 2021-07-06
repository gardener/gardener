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
	"context"

	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	fakeclientset "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	baseconfig "k8s.io/component-base/config"
)

var _ = Describe("SeedClientMapBuilder", func() {

	var (
		ctx context.Context

		fakeLogger          logrus.FieldLogger
		fakeGardenClientMap *fakeclientmap.ClientMap
		fakeGardenClientSet *fakeclientset.ClientSet

		clientConnectionConfig *baseconfig.ClientConnectionConfiguration
	)

	BeforeEach(func() {
		ctx = context.TODO()

		fakeLogger = logger.NewNopLogger()
		fakeGardenClientSet = fakeclientset.NewClientSet()
		fakeGardenClientMap = fakeclientmap.NewClientMapBuilder().WithClientSetForKey(keys.ForGarden(), fakeGardenClientSet).Build()

		clientConnectionConfig = &baseconfig.ClientConnectionConfiguration{}
	})

	Context("#logger", func() {
		It("should be correctly set by WithLogger", func() {
			builder := NewSeedClientMapBuilder().WithLogger(fakeLogger)
			Expect(builder.logger).To(BeEquivalentTo(fakeLogger))
		})
	})

	Context("#gardenClientFunc", func() {
		It("should be correctly set by WithGardenClientSet", func() {
			builder := NewSeedClientMapBuilder().WithGardenClientSet(fakeGardenClientSet)
			Expect(builder.gardenClientFunc(ctx)).To(BeEquivalentTo(fakeGardenClientSet))
		})

		It("should be correctly set by WithGardenClientMap", func() {
			builder := NewSeedClientMapBuilder().WithGardenClientMap(fakeGardenClientMap)
			Expect(builder.gardenClientFunc(ctx)).To(BeEquivalentTo(fakeGardenClientSet))
		})
	})

	Context("#clientConnectionConfig", func() {
		It("should be correctly set by WithClientConnectionConfig", func() {
			builder := NewSeedClientMapBuilder().WithClientConnectionConfig(clientConnectionConfig)
			Expect(builder.clientConnectionConfig).To(BeIdenticalTo(clientConnectionConfig))
		})
	})

	Context("#Build", func() {
		It("should fail if logger was not set", func() {
			clientMap, err := NewSeedClientMapBuilder().Build()
			Expect(err).To(MatchError("logger is required but not set"))
			Expect(clientMap).To(BeNil())
		})

		It("should fail if garden ClientMap was not set", func() {
			clientMap, err := NewSeedClientMapBuilder().WithLogger(fakeLogger).Build()
			Expect(err).To(MatchError("garden client is required but not set"))
			Expect(clientMap).To(BeNil())
		})

		It("should fail if clientConnectionConfig was not set", func() {
			clientMap, err := NewSeedClientMapBuilder().WithLogger(fakeLogger).WithGardenClientSet(fakeGardenClientSet).Build()
			Expect(err).To(MatchError("clientConnectionConfig is required but not set"))
			Expect(clientMap).To(BeNil())
		})

		It("should succeed to build ClientMap", func() {
			clientSet, err := NewSeedClientMapBuilder().
				WithLogger(fakeLogger).
				WithGardenClientMap(fakeGardenClientMap).
				WithClientConnectionConfig(clientConnectionConfig).
				Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(clientSet).NotTo(BeNil())
		})
	})

})
