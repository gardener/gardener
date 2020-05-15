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

	"k8s.io/apimachinery/pkg/version"

	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/internal"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/client/kubernetes/test"
	"github.com/gardener/gardener/pkg/logger"
	mockkubernetes "github.com/gardener/gardener/pkg/mock/gardener/client/kubernetes"
	mockclientmap "github.com/gardener/gardener/pkg/mock/gardener/client/kubernetes/clientmap"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			factory = mockclientmap.NewMockClientSetFactory(ctrl)
			cs = mockkubernetes.NewMockInterface(ctrl)
			csVersion = &version.Info{GitVersion: "1.18.0"}
			cs.EXPECT().Version().Return(csVersion.GitVersion).AnyTimes()

			cm = internal.NewGenericClientMap(factory, logger.NewNopLogger())
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		Context("#GetClient", func() {
			It("should create a new ClientSet (clientMap empty)", func() {
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, nil)

				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				cs.EXPECT().Kubernetes().Return(test.NewClientSetWithFakedServerVersion(nil, csVersion))
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs), "should return the ClientSet already contained in the ClientMap")
			})

			It("should failed to create a new ClientSet if factory fails", func() {
				fakeErr := fmt.Errorf("fake")
				factory.EXPECT().NewClientSet(ctx, key).Return(nil, fakeErr)

				cs, err := cm.GetClient(ctx, key)
				Expect(cs).To(BeNil())
				Expect(err).To(MatchError(fmt.Sprintf("error creating new ClientSet for key %q: %v", key.Key(), fakeErr)))
			})

			It("should create a new ClientSet and start it automatically", func() {
				Expect(cm.Start(ctx.Done())).To(Succeed())

				gomock.InOrder(
					factory.EXPECT().NewClientSet(ctx, key).Return(cs, nil),
					cs.EXPECT().Start(gomock.Any()),
					cs.EXPECT().WaitForCacheSync(gomock.Any()).Return(true),
				)

				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				cs.EXPECT().Kubernetes().Return(test.NewClientSetWithFakedServerVersion(nil, csVersion))
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs), "should return the ClientSet already contained in the ClientMap")
			})

			It("should create a new ClientSet and fail because cache cannot be synced", func() {
				Expect(cm.Start(ctx.Done())).To(Succeed())

				gomock.InOrder(
					factory.EXPECT().NewClientSet(ctx, key).Return(cs, nil),
					cs.EXPECT().Start(gomock.Any()),
					cs.EXPECT().WaitForCacheSync(gomock.Any()).Return(false),
				)

				cs, err := cm.GetClient(ctx, key)
				Expect(cs).To(BeNil())
				Expect(err).To(MatchError(fmt.Sprintf("timed out waiting for caches of ClientSet with key %q to sync", key.Key())))
			})

			It("should create a new ClientSet after version change", func() {
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, nil)

				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				// fake version change for second call to GetClient
				cs.EXPECT().Kubernetes().Return(test.NewClientSetWithFakedServerVersion(nil, &version.Info{GitVersion: "1.18.1"}))

				cs2 := mockkubernetes.NewMockInterface(ctrl)
				factory.EXPECT().NewClientSet(ctx, key).Return(cs2, nil)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs2), "should return a new ClientSet as server version has changed")

				// fake version change for third call to GetClient
				cs2Version := &version.Info{GitVersion: "1.18.2"}
				cs2.EXPECT().Version().Return(cs2Version.GitVersion).AnyTimes()
				cs2.EXPECT().Kubernetes().Return(test.NewClientSetWithFakedServerVersion(nil, cs2Version))
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs2), "should return the same ClientSet if the server version has changed a second time shortly afterwards")
			})
		})

		Context("#InvalidateClient", func() {
			It("should do nothing if matching ClientSet is not found", func() {
				Expect(cm.InvalidateClient(key)).To(Succeed())
			})

			It("should delete the matching ClientSet from the ClientMap", func() {
				By("should create a new ClientSet beforehand")
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, nil)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				By("should remove the matching ClientSet")
				Expect(cm.InvalidateClient(key)).To(Succeed())

				By("should need to create a new ClientSet afterwards")
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, nil)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))
			})

			It("should delete the matching ClientSet from the ClientMap and cancel its context", func() {
				Expect(cm.Start(ctx.Done())).To(Succeed())

				var clientSetStopCh <-chan struct{}

				By("should create a new ClientSet beforehand and start it automatically")
				gomock.InOrder(
					factory.EXPECT().NewClientSet(ctx, key).Return(cs, nil),
					cs.EXPECT().Start(gomock.Any()).Do(func(stopCh <-chan struct{}) {
						clientSetStopCh = stopCh
					}),
					cs.EXPECT().WaitForCacheSync(gomock.Any()).Return(true),
				)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				By("should remove the matching ClientSet and cancel its context")
				Expect(cm.InvalidateClient(key)).To(Succeed())
				Expect(clientSetStopCh).To(BeClosed())

				By("should need to create a new ClientSet afterwards")
				gomock.InOrder(
					factory.EXPECT().NewClientSet(ctx, key).Return(cs, nil),
					cs.EXPECT().Start(gomock.Any()),
					cs.EXPECT().WaitForCacheSync(gomock.Any()).Return(true),
				)
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

			It("should start ClientSets already contained in the ClientMap", func() {
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, nil)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				cs.EXPECT().Start(gomock.Any())
				Expect(cm.Start(ctx.Done())).To(Succeed())
			})
		})
	})
})
