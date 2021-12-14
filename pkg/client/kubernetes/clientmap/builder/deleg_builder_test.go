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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
	baseconfig "k8s.io/component-base/config"

	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
)

var _ = Describe("DelegatingClientMapBuilder", func() {

	var (
		fakeGardenClientMap *fakeclientmap.ClientMap
		fakeSeedClientMap   *fakeclientmap.ClientMap
		fakeShootClientMap  *fakeclientmap.ClientMap
		fakePlantClientMap  *fakeclientmap.ClientMap
	)

	BeforeEach(func() {
		fakeGardenClientMap = fakeclientmap.NewClientMap()
		fakeSeedClientMap = fakeclientmap.NewClientMap()
		fakeShootClientMap = fakeclientmap.NewClientMap()
		fakePlantClientMap = fakeclientmap.NewClientMap()
	})

	Context("#gardenClientMapFunc", func() {
		It("should be set correctly by WithGardenClientMap", func() {
			builder := NewDelegatingClientMapBuilder().WithGardenClientMap(fakeGardenClientMap)
			Expect(builder.gardenClientMapFunc()).To(BeIdenticalTo(fakeGardenClientMap))
		})

		It("should be set correctly by WithGardenClientMapBuilder", func() {
			clientMap, err := NewDelegatingClientMapBuilder().
				WithGardenClientMapBuilder(NewGardenClientMapBuilder().WithRESTConfig(&rest.Config{})).
				Build()

			Expect(err).NotTo(HaveOccurred())
			Expect(clientMap).NotTo(BeNil())
		})
	})

	Context("#seedClientMapFunc", func() {
		It("should be set correctly by WithSeedClientMap", func() {
			builder := NewDelegatingClientMapBuilder().WithSeedClientMap(fakeSeedClientMap)
			Expect(builder.seedClientMapFunc()).To(BeIdenticalTo(fakeSeedClientMap))
		})

		It("should be set correctly by WithSeedClientMapBuilder", func() {
			clientMap, err := NewDelegatingClientMapBuilder().
				WithGardenClientMap(fakeGardenClientMap).
				WithSeedClientMapBuilder(NewSeedClientMapBuilder().WithClientConnectionConfig(&baseconfig.ClientConnectionConfiguration{})).
				Build()

			Expect(err).NotTo(HaveOccurred())
			Expect(clientMap).NotTo(BeNil())
		})
	})

	Context("#shootClientMapFunc", func() {
		It("should be set correctly by WithShootClientMap", func() {
			builder := NewDelegatingClientMapBuilder().WithShootClientMap(fakeShootClientMap)
			Expect(builder.shootClientMapFunc(nil, nil)).To(BeIdenticalTo(fakeShootClientMap))
		})

		It("should be set correctly by WithShootClientMapBuilder", func() {
			clientMap, err := NewDelegatingClientMapBuilder().
				WithGardenClientMap(fakeGardenClientMap).
				WithSeedClientMap(fakeSeedClientMap).
				WithShootClientMapBuilder(NewShootClientMapBuilder().WithClientConnectionConfig(&baseconfig.ClientConnectionConfiguration{})).
				Build()

			Expect(err).NotTo(HaveOccurred())
			Expect(clientMap).NotTo(BeNil())
		})
	})

	Context("#plantClientMapFunc", func() {
		It("should be set correctly by WithPlantClientMap", func() {
			builder := NewDelegatingClientMapBuilder().WithPlantClientMap(fakePlantClientMap)
			Expect(builder.plantClientMapFunc(nil)).To(BeIdenticalTo(fakePlantClientMap))
		})

		It("should be set correctly by WithPlantClientMapBuilder", func() {
			clientMap, err := NewDelegatingClientMapBuilder().
				WithGardenClientMap(fakeGardenClientMap).
				WithPlantClientMapBuilder(NewPlantClientMapBuilder()).
				Build()

			Expect(err).NotTo(HaveOccurred())
			Expect(clientMap).NotTo(BeNil())
		})
	})

	Context("#Build", func() {
		It("should fail if gardenClientMapFunc was not set", func() {
			clientMap, err := NewDelegatingClientMapBuilder().Build()
			Expect(err).To(MatchError(ContainSubstring("failed to construct garden ClientMap")))
			Expect(clientMap).To(BeNil())
		})

		It("should fail if gardenClientMapFunc fails", func() {
			fakeErr := fmt.Errorf("fake")
			builder := NewDelegatingClientMapBuilder()
			builder.gardenClientMapFunc = func() (clientmap.ClientMap, error) {
				return nil, fakeErr
			}
			clientMap, err := builder.Build()
			Expect(err).To(MatchError(ContainSubstring("failed to construct garden ClientMap")))
			Expect(clientMap).To(BeNil())
		})

		It("should fail if seedClientMapFunc fails", func() {
			fakeErr := fmt.Errorf("fake")
			builder := NewDelegatingClientMapBuilder().
				WithGardenClientMap(fakeGardenClientMap)
			builder.seedClientMapFunc = func() (clientmap.ClientMap, error) {
				return nil, fakeErr
			}
			clientMap, err := builder.Build()
			Expect(err).To(MatchError(ContainSubstring("failed to construct seed ClientMap")))
			Expect(clientMap).To(BeNil())
		})

		It("should fail if shootClientMapFunc is set but seedClientMapFunc is not", func() {
			fakeErr := fmt.Errorf("fake")
			builder := NewDelegatingClientMapBuilder().
				WithGardenClientMap(fakeGardenClientMap)
			builder.shootClientMapFunc = func(clientmap.ClientMap, clientmap.ClientMap) (clientmap.ClientMap, error) {
				return nil, fakeErr
			}
			clientMap, err := builder.Build()
			Expect(err).To(MatchError(ContainSubstring("seed ClientMap is required but not set")))
			Expect(clientMap).To(BeNil())
		})

		It("should fail if shootClientMapFunc fails", func() {
			fakeErr := fmt.Errorf("fake")
			builder := NewDelegatingClientMapBuilder().
				WithGardenClientMap(fakeGardenClientMap).
				WithSeedClientMap(fakeSeedClientMap)
			builder.shootClientMapFunc = func(clientmap.ClientMap, clientmap.ClientMap) (clientmap.ClientMap, error) {
				return nil, fakeErr
			}
			clientMap, err := builder.Build()
			Expect(err).To(MatchError(ContainSubstring("failed to construct shoot ClientMap")))
			Expect(clientMap).To(BeNil())
		})

		It("should fail if plantClientMapFunc fails", func() {
			fakeErr := fmt.Errorf("fake")
			builder := NewDelegatingClientMapBuilder().
				WithGardenClientMap(fakeGardenClientMap)
			builder.plantClientMapFunc = func(clientmap.ClientMap) (clientmap.ClientMap, error) {
				return nil, fakeErr
			}
			clientMap, err := builder.Build()
			Expect(err).To(MatchError(ContainSubstring("failed to construct plant ClientMap")))
			Expect(clientMap).To(BeNil())
		})

		It("should succeed to build ClientMap", func() {
			clientMap, err := NewDelegatingClientMapBuilder().
				WithGardenClientMap(fakeGardenClientMap).
				WithSeedClientMap(fakeSeedClientMap).
				Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(clientMap).NotTo(BeNil())
		})
	})

})
