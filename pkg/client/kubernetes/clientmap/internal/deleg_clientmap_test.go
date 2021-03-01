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

package internal_test

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/internal"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	mockclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/mock"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DelegatingClientMap", func() {
	var (
		ctx  context.Context
		cm   clientmap.ClientMap
		key  clientmap.ClientSetKey
		ctrl *gomock.Controller

		gardenClientMap, seedClientMap, shootClientMap, plantClientMap *mockclientmap.MockClientMap
	)

	BeforeEach(func() {
		ctx = context.TODO()
		ctrl = gomock.NewController(GinkgoT())

		gardenClientMap = mockclientmap.NewMockClientMap(ctrl)
		seedClientMap = mockclientmap.NewMockClientMap(ctrl)
		shootClientMap = mockclientmap.NewMockClientMap(ctrl)
		plantClientMap = mockclientmap.NewMockClientMap(ctrl)

		cm = internal.NewDelegatingClientMap(gardenClientMap, seedClientMap, shootClientMap, plantClientMap)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("NewDelegatingClientMap", func() {
		It("should panic, if gardenClientMap is nil", func() {
			Expect(func() {
				_ = internal.NewDelegatingClientMap(nil, nil, nil, nil)
			}).To(Panic())
		})
	})

	Context("GardenClientSetKey", func() {
		BeforeEach(func() {
			key = keys.ForGarden()
		})

		It("Should delegate GetClient to GardenClientMap", func() {
			gardenClientMap.EXPECT().GetClient(ctx, key).Return(nil, nil)
			_, err := cm.GetClient(ctx, key)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should delegate InvalidateClient to GardenClientMap", func() {
			gardenClientMap.EXPECT().InvalidateClient(key)
			Expect(cm.InvalidateClient(key)).To(Succeed())
		})
	})

	Context("SeedClientSetKey", func() {
		BeforeEach(func() {
			key = keys.ForSeedWithName("eu-1")
		})

		It("Should delegate GetClient to SeedClientMap", func() {
			seedClientMap.EXPECT().GetClient(ctx, key).Return(nil, nil)
			_, err := cm.GetClient(ctx, key)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should error on GetClient if SeedClientMap is nil", func() {
			cm = internal.NewDelegatingClientMap(gardenClientMap, nil, shootClientMap, plantClientMap)
			_, err := cm.GetClient(ctx, key)
			Expect(err).To(MatchError(ContainSubstring("unsupported ClientSetKey type")))
		})

		It("Should delegate InvalidateClient to SeedClientMap", func() {
			seedClientMap.EXPECT().InvalidateClient(key)
			Expect(cm.InvalidateClient(key)).To(Succeed())
		})

		It("Should error on InvalidateClient if SeedClientMap is nil", func() {
			cm = internal.NewDelegatingClientMap(gardenClientMap, nil, shootClientMap, plantClientMap)
			err := cm.InvalidateClient(key)
			Expect(err).To(MatchError(ContainSubstring("unsupported ClientSetKey type")))
		})
	})

	Context("ShootClientSetKey", func() {
		BeforeEach(func() {
			key = keys.ForShootWithNamespacedName("core", "sunflower")
		})

		It("Should delegate GetClient to ShootClientMap", func() {
			shootClientMap.EXPECT().GetClient(ctx, key).Return(nil, nil)
			_, err := cm.GetClient(ctx, key)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should error on GetClient if ShootClientMap is nil", func() {
			cm = internal.NewDelegatingClientMap(gardenClientMap, seedClientMap, nil, plantClientMap)
			_, err := cm.GetClient(ctx, key)
			Expect(err).To(MatchError(ContainSubstring("unsupported ClientSetKey type")))
		})

		It("Should delegate InvalidateClient to ShootClientMap", func() {
			shootClientMap.EXPECT().InvalidateClient(key)
			Expect(cm.InvalidateClient(key)).To(Succeed())
		})

		It("Should error on InvalidateClient if ShootClientMap is nil", func() {
			cm = internal.NewDelegatingClientMap(gardenClientMap, seedClientMap, nil, plantClientMap)
			err := cm.InvalidateClient(key)
			Expect(err).To(MatchError(ContainSubstring("unsupported ClientSetKey type")))
		})
	})

	Context("PlantClientSetKey", func() {
		BeforeEach(func() {
			key = keys.ForPlantWithNamespacedName("core", "lotus")
		})

		It("Should delegate GetClient to PlantClientMap", func() {
			plantClientMap.EXPECT().GetClient(ctx, key).Return(nil, nil)
			_, err := cm.GetClient(ctx, key)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should error on GetClient if PlantClientMap is nil", func() {
			cm = internal.NewDelegatingClientMap(gardenClientMap, seedClientMap, shootClientMap, nil)
			_, err := cm.GetClient(ctx, key)
			Expect(err).To(MatchError(ContainSubstring("unsupported ClientSetKey type")))
		})

		It("Should delegate InvalidateClient to PlantClientMap", func() {
			plantClientMap.EXPECT().InvalidateClient(key)
			Expect(cm.InvalidateClient(key)).To(Succeed())
		})

		It("Should error on InvalidateClient if PlantClientMap is nil", func() {
			cm = internal.NewDelegatingClientMap(gardenClientMap, seedClientMap, shootClientMap, nil)
			err := cm.InvalidateClient(key)
			Expect(err).To(MatchError(ContainSubstring("unsupported ClientSetKey type")))
		})
	})

	Describe("#GetClient", func() {
		It("should fail for unknown ClientSetKey type", func() {
			key = fakeKey{}
			cs, err := cm.GetClient(ctx, key)
			Expect(cs).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("unknown ClientSetKey type")))
		})
	})

	Describe("#InvalidateClient", func() {
		It("should fail for unknown ClientSetKey type", func() {
			key = fakeKey{}
			err := cm.InvalidateClient(key)
			Expect(err).To(MatchError(ContainSubstring("unknown ClientSetKey type")))
		})
	})

	Describe("#Start", func() {
		It("should delegate start to all ClientMaps", func() {
			gardenClientMap.EXPECT().Start(ctx.Done())
			seedClientMap.EXPECT().Start(ctx.Done())
			shootClientMap.EXPECT().Start(ctx.Done())
			plantClientMap.EXPECT().Start(ctx.Done())

			Expect(cm.Start(ctx.Done())).To(Succeed())
		})

		It("should fail, as starting GardenClients fails", func() {
			fakeErr := fmt.Errorf("fake")
			gardenClientMap.EXPECT().Start(ctx.Done()).Return(fakeErr)
			Expect(cm.Start(ctx.Done())).To(MatchError("failed to start garden ClientMap: fake"))
		})

		It("should fail, as starting SeedClients fails", func() {
			fakeErr := fmt.Errorf("fake")
			gardenClientMap.EXPECT().Start(ctx.Done())
			seedClientMap.EXPECT().Start(ctx.Done()).Return(fakeErr)
			Expect(cm.Start(ctx.Done())).To(MatchError("failed to start seed ClientMap: fake"))
		})

		It("should fail, as starting ShootClients fails", func() {
			fakeErr := fmt.Errorf("fake")
			gardenClientMap.EXPECT().Start(ctx.Done())
			seedClientMap.EXPECT().Start(ctx.Done())
			shootClientMap.EXPECT().Start(ctx.Done()).Return(fakeErr)
			Expect(cm.Start(ctx.Done())).To(MatchError("failed to start shoot ClientMap: fake"))
		})

		It("should fail, as starting PlantClients fails", func() {
			fakeErr := fmt.Errorf("fake")
			gardenClientMap.EXPECT().Start(ctx.Done())
			seedClientMap.EXPECT().Start(ctx.Done())
			shootClientMap.EXPECT().Start(ctx.Done())
			plantClientMap.EXPECT().Start(ctx.Done()).Return(fakeErr)
			Expect(cm.Start(ctx.Done())).To(MatchError("failed to start plant ClientMap: fake"))
		})

	})

})

type fakeKey struct{}

func (f fakeKey) Key() string {
	return "fake"
}
