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
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/internal"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	mockclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/mock"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/version"
)

var _ = Describe("GenericClientMap", func() {
	var (
		ctx context.Context
		cm  *internal.GenericClientMap
		key clientmap.ClientSetKey
	)

	BeforeEach(func() {
		ctx = context.TODO()
		key = keys.ForGarden()
	})

	Context("initialized ClientMap", func() {
		var (
			ctrl    *gomock.Controller
			factory *mockclientmap.MockClientSetFactory
			cs      *mockkubernetes.MockInterface

			csVersion *version.Info

			origMaxRefreshInterval time.Duration
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			factory = mockclientmap.NewMockClientSetFactory(ctrl)
			cs = mockkubernetes.NewMockInterface(ctrl)
			csVersion = &version.Info{GitVersion: "1.18.0"}
			cs.EXPECT().Version().Return(csVersion.GitVersion).AnyTimes()

			cm = internal.NewGenericClientMap(factory, logger.NewNopLogger())

			origMaxRefreshInterval = internal.MaxRefreshInterval
			internal.MaxRefreshInterval = 10 * time.Millisecond
		})

		AfterEach(func() {
			ctrl.Finish()
			internal.MaxRefreshInterval = origMaxRefreshInterval
		})

		Context("#GetClient", func() {
			It("should create a new ClientSet (clientMap empty)", func() {
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, nil)
				factory.EXPECT().CalculateClientSetHash(ctx, key).Return("", nil)

				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs), "should return the ClientSet already contained in the ClientMap")
			})

			It("should failed to create a new ClientSet if factory fails", func() {
				fakeErr := fmt.Errorf("fake")
				factory.EXPECT().NewClientSet(ctx, key).Return(nil, fakeErr)

				clientSet, err := cm.GetClient(ctx, key)
				Expect(clientSet).To(BeNil())
				Expect(err).To(MatchError(fmt.Sprintf("error creating new ClientSet for key %q: %v", key.Key(), fakeErr)))
			})

			It("should create a new ClientSet and start it automatically", func() {
				Expect(cm.Start(ctx.Done())).To(Succeed())

				factory.EXPECT().NewClientSet(ctx, key).Return(cs, nil)
				factory.EXPECT().CalculateClientSetHash(ctx, key).Return("", nil)
				cs.EXPECT().Start(gomock.Any())
				cs.EXPECT().WaitForCacheSync(gomock.Any()).Return(true)

				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs), "should return the ClientSet already contained in the ClientMap")
			})

			It("should create a new ClientSet and fail because cache cannot be synced", func() {
				Expect(cm.Start(ctx.Done())).To(Succeed())

				factory.EXPECT().NewClientSet(ctx, key).Return(cs, nil)
				factory.EXPECT().CalculateClientSetHash(ctx, key).Return("", nil)
				cs.EXPECT().Start(gomock.Any())
				cs.EXPECT().WaitForCacheSync(gomock.Any()).Return(false)

				clientSet, err := cm.GetClient(ctx, key)
				Expect(clientSet).To(BeNil())
				Expect(err).To(MatchError(fmt.Sprintf("timed out waiting for caches of ClientSet with key %q to sync", key.Key())))
			})

			It("should refresh the ClientSet's server version", func() {
				By("should create a new ClientSet")
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, nil)
				factory.EXPECT().CalculateClientSetHash(ctx, key).Return("", nil).AnyTimes()
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				By("should not check for a version change directly after creating the client")
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				By("should refresh the ClientSet's server version")
				// let the max refresh interval pass
				time.Sleep(internal.MaxRefreshInterval)
				cs.EXPECT().DiscoverVersion().Return(&version.Info{GitVersion: "1.18.1"}, nil)
				clientSet, err := cm.GetClient(ctx, key)
				Expect(clientSet).To(BeIdenticalTo(cs))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail to refresh the ClientSet's server version because DiscoverVersion fails", func() {
				By("should create a new ClientSet")
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, nil)
				factory.EXPECT().CalculateClientSetHash(ctx, key).Return("", nil).AnyTimes()
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				By("should not check for a version change directly after creating the client")
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				By("should fail to refresh the ClientSet's server version because DiscoverVersion fails")
				// let the max refresh interval pass
				time.Sleep(internal.MaxRefreshInterval)
				cs.EXPECT().DiscoverVersion().Return(nil, fmt.Errorf("fake"))
				clientSet, err := cm.GetClient(ctx, key)
				Expect(clientSet).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("fake")))
			})

			It("should refresh the ClientSet because of hash change", func() {
				By("should create a new ClientSet")
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, nil)
				factory.EXPECT().CalculateClientSetHash(ctx, key).Return("hash1", nil)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				By("should not check for a hash change directly after creating the client")
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				By("should not refresh the ClientSet as version and hash haven't changed")
				// let the max refresh interval pass
				time.Sleep(internal.MaxRefreshInterval)
				cs.EXPECT().DiscoverVersion().Return(csVersion, nil)
				factory.EXPECT().CalculateClientSetHash(ctx, key).Return("hash1", nil)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				By("should refresh the ClientSet as the hash has changed")
				// let the max refresh interval pass again
				time.Sleep(internal.MaxRefreshInterval)
				cs.EXPECT().DiscoverVersion().Return(csVersion, nil)
				factory.EXPECT().CalculateClientSetHash(ctx, key).Return("hash2", nil).Times(2)

				cs2 := mockkubernetes.NewMockInterface(ctrl)
				factory.EXPECT().NewClientSet(ctx, key).Return(cs2, nil)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs2))
			})

			It("should fail because CalculateClientSetHash fails for new ClientSet", func() {
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, nil)
				factory.EXPECT().CalculateClientSetHash(ctx, key).Return("", fmt.Errorf("fake"))

				clientSet, err := cm.GetClient(ctx, key)
				Expect(clientSet).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("fake")))
			})

			It("should fail because CalculateClientSetHash fails for existing ClientSet", func() {
				By("should create a new ClientSet")
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, nil)
				factory.EXPECT().CalculateClientSetHash(ctx, key).Return("hash1", nil)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				By("should fail to get the ClientSet again because CalculateClientSetHash fails")
				// let the max refresh interval pass again
				time.Sleep(internal.MaxRefreshInterval)
				cs.EXPECT().DiscoverVersion().Return(csVersion, nil)
				factory.EXPECT().CalculateClientSetHash(ctx, key).Return("", fmt.Errorf("fake"))

				clientSet, err := cm.GetClient(ctx, key)
				Expect(clientSet).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("fake")))
			})
		})

		Context("#InvalidateClient", func() {
			It("should do nothing if matching ClientSet is not found", func() {
				Expect(cm.InvalidateClient(key)).To(Succeed())
			})

			It("should delete the matching ClientSet from the ClientMap", func() {
				By("should create a new ClientSet beforehand")
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, nil)
				factory.EXPECT().CalculateClientSetHash(ctx, key).Return("", nil)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				By("should remove the matching ClientSet")
				Expect(cm.InvalidateClient(key)).To(Succeed())

				By("should need to create a new ClientSet afterwards")
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, nil)
				factory.EXPECT().CalculateClientSetHash(ctx, key).Return("", nil)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))
			})

			It("should delete the matching ClientSet from the ClientMap and cancel its context", func() {
				Expect(cm.Start(ctx.Done())).To(Succeed())

				var clientSetStopCh <-chan struct{}

				By("should create a new ClientSet beforehand and start it automatically")
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, nil)
				factory.EXPECT().CalculateClientSetHash(ctx, key).Return("", nil)
				cs.EXPECT().Start(gomock.Any()).Do(func(ctx context.Context) {
					clientSetStopCh = ctx.Done()
				})
				cs.EXPECT().WaitForCacheSync(gomock.Any()).Return(true)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				By("should remove the matching ClientSet and cancel its context")
				Expect(cm.InvalidateClient(key)).To(Succeed())
				Expect(clientSetStopCh).To(BeClosed())

				By("should need to create a new ClientSet afterwards")
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, nil)
				factory.EXPECT().CalculateClientSetHash(ctx, key).Return("", nil)
				cs.EXPECT().Start(gomock.Any())
				cs.EXPECT().WaitForCacheSync(gomock.Any()).Return(true)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))
			})
		})

		Context("#Start", func() {
			It("should do nothing if the ClientMap is empty", func() {
				Expect(cm.Start(ctx.Done())).To(Succeed())
			})

			It("should do nothing if the ClientMap is already started", func() {
				Expect(cm.Start(ctx.Done())).To(Succeed())
				Expect(cm.Start(ctx.Done())).To(Succeed())
			})

			It("should start ClientSets already contained in the ClientMap and wait for caches to sync", func() {
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, nil)
				factory.EXPECT().CalculateClientSetHash(ctx, key).Return("", nil)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				cs.EXPECT().Start(gomock.Any())
				cs.EXPECT().WaitForCacheSync(gomock.Any()).Return(true)
				Expect(cm.Start(ctx.Done())).To(Succeed())
			})

			It("should fail if caches cannot be synced", func() {
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, nil)
				factory.EXPECT().CalculateClientSetHash(ctx, key).Return("", nil)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				cs.EXPECT().Start(gomock.Any())
				cs.EXPECT().WaitForCacheSync(gomock.Any()).Return(false)
				Expect(cm.Start(ctx.Done())).To(MatchError(ContainSubstring("timed out waiting for caches of ClientSet")))
			})
		})
	})
})
