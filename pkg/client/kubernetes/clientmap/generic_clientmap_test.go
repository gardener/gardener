// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package clientmap_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/version"
	testclock "k8s.io/utils/clock/testing"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	mockclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/mock"
	kubernetesmock "github.com/gardener/gardener/pkg/client/kubernetes/mock"
)

var _ = Describe("GenericClientMap", func() {
	var (
		ctx context.Context
		cm  *GenericClientMap
		key ClientSetKey
	)

	BeforeEach(func() {
		ctx = context.TODO()
		key = keys.ForShoot(&gardencorev1beta1.Shoot{})
	})

	Context("initialized ClientMap", func() {
		var (
			ctrl    *gomock.Controller
			factory *mockclientmap.MockClientSetFactory
			cs      *kubernetesmock.MockInterface

			csVersion *version.Info

			origMaxRefreshInterval time.Duration

			fakeClock *testclock.FakeClock
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			factory = mockclientmap.NewMockClientSetFactory(ctrl)
			cs = kubernetesmock.NewMockInterface(ctrl)
			csVersion = &version.Info{GitVersion: "1.27.0"}
			cs.EXPECT().Version().Return(csVersion.GitVersion).AnyTimes()

			fakeClock = testclock.NewFakeClock(time.Now())
			cm = NewGenericClientMap(factory, logr.Discard(), fakeClock)

			origMaxRefreshInterval = MaxRefreshInterval
			MaxRefreshInterval = 10 * time.Millisecond
		})

		AfterEach(func() {
			ctrl.Finish()
			MaxRefreshInterval = origMaxRefreshInterval
		})

		Context("#GetClient", func() {
			It("should create a new ClientSet (clientMap empty)", func() {
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, "", nil)

				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs), "should return the ClientSet already contained in the ClientMap")
			})

			It("should failed to create a new ClientSet if factory fails", func() {
				fakeErr := errors.New("fake")
				factory.EXPECT().NewClientSet(ctx, key).Return(nil, "", fakeErr)

				clientSet, err := cm.GetClient(ctx, key)
				Expect(clientSet).To(BeNil())
				Expect(err).To(MatchError(fmt.Sprintf("error creating new ClientSet for key %q: %v", key.Key(), fakeErr)))
			})

			It("should create a new ClientSet and start it automatically", func() {
				Expect(cm.Start(ctx)).To(Succeed())

				factory.EXPECT().NewClientSet(ctx, key).Return(cs, "", nil)
				cs.EXPECT().Start(gomock.Any())
				cs.EXPECT().WaitForCacheSync(gomock.Any()).Return(true)

				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs), "should return the ClientSet already contained in the ClientMap")
			})

			It("should create a new ClientSet and fail because cache cannot be synced", func() {
				Expect(cm.Start(ctx)).To(Succeed())

				factory.EXPECT().NewClientSet(ctx, key).Return(cs, "", nil)
				cs.EXPECT().Start(gomock.Any())
				cs.EXPECT().WaitForCacheSync(gomock.Any()).Return(false)

				clientSet, err := cm.GetClient(ctx, key)
				Expect(clientSet).To(BeNil())
				Expect(err).To(MatchError(fmt.Sprintf("timed out waiting for caches of ClientSet with key %q to sync", key.Key())))
			})

			It("should refresh the ClientSet's server version", func() {
				By("Should create a new ClientSet")
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, "", nil)
				factory.EXPECT().CalculateClientSetHash(ctx, key).Return("", nil).AnyTimes()
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				By("Should not check for a version change directly after creating the client")
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				By("Should refresh the ClientSet's server version")
				// let the max refresh interval pass
				fakeClock.Sleep(MaxRefreshInterval)
				cs.EXPECT().DiscoverVersion().Return(&version.Info{GitVersion: "1.27.1"}, nil)
				clientSet, err := cm.GetClient(ctx, key)
				Expect(clientSet).To(BeIdenticalTo(cs))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fail to refresh the ClientSet's server version because DiscoverVersion fails", func() {
				By("Should create a new ClientSet")
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, "", nil)
				factory.EXPECT().CalculateClientSetHash(ctx, key).Return("", nil).AnyTimes()
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				By("Should not check for a version change directly after creating the client")
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				By("Should fail to refresh the ClientSet's server version because DiscoverVersion fails")
				// let the max refresh interval pass
				fakeClock.Sleep(MaxRefreshInterval)
				cs.EXPECT().DiscoverVersion().Return(nil, errors.New("fake"))
				clientSet, err := cm.GetClient(ctx, key)
				Expect(clientSet).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("fake")))
			})

			It("should refresh the ClientSet because of hash change", func() {
				By("Should create a new ClientSet")
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, "hash1", nil)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				By("Should not check for a hash change directly after creating the client")
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				By("Should not refresh the ClientSet as version and hash haven't changed")
				// let the max refresh interval pass
				fakeClock.Sleep(MaxRefreshInterval)
				factory.EXPECT().CalculateClientSetHash(ctx, key).Return("hash1", nil)
				cs.EXPECT().DiscoverVersion().Return(csVersion, nil)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				By("Should refresh the ClientSet as the hash has changed")
				// let the max refresh interval pass again
				fakeClock.Sleep(MaxRefreshInterval)
				factory.EXPECT().CalculateClientSetHash(ctx, key).Return("hash2", nil)

				cs2 := kubernetesmock.NewMockInterface(ctrl)
				factory.EXPECT().NewClientSet(ctx, key).Return(cs2, "hash2", nil)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs2))
			})

			It("should fail because CalculateClientSetHash fails for existing ClientSet", func() {
				By("Should create a new ClientSet")
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, "hash1", nil)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				By("Should fail to get the ClientSet again because CalculateClientSetHash fails")
				// let the max refresh interval pass again
				fakeClock.Sleep(MaxRefreshInterval)
				factory.EXPECT().CalculateClientSetHash(ctx, key).Return("", errors.New("fake"))

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
				By("Should create a new ClientSet beforehand")
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, "", nil)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				By("Should remove the matching ClientSet")
				Expect(cm.InvalidateClient(key)).To(Succeed())

				By("Should need to create a new ClientSet afterwards")
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, "", nil)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))
			})

			It("should delete the matching ClientSet from the ClientMap and cancel its context", func() {
				Expect(cm.Start(ctx)).To(Succeed())

				var clientSetStopCh <-chan struct{}

				By("Should create a new ClientSet beforehand and start it automatically")
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, "", nil)
				cs.EXPECT().Start(gomock.Any()).Do(func(ctx context.Context) {
					clientSetStopCh = ctx.Done()
				})
				cs.EXPECT().WaitForCacheSync(gomock.Any()).Return(true)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				By("Should remove the matching ClientSet and cancel its context")
				Expect(cm.InvalidateClient(key)).To(Succeed())
				Expect(clientSetStopCh).To(BeClosed())

				By("Should need to create a new ClientSet afterwards")
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, "", nil)
				cs.EXPECT().Start(gomock.Any())
				cs.EXPECT().WaitForCacheSync(gomock.Any()).Return(true)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))
			})
		})

		Context("#Start", func() {
			It("should do nothing if the ClientMap is empty", func() {
				Expect(cm.Start(ctx)).To(Succeed())
			})

			It("should do nothing if the ClientMap is already started", func() {
				Expect(cm.Start(ctx)).To(Succeed())
				Expect(cm.Start(ctx)).To(Succeed())
			})

			It("should start ClientSets already contained in the ClientMap and wait for caches to sync", func() {
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, "", nil)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				cs.EXPECT().Start(gomock.Any())
				cs.EXPECT().WaitForCacheSync(gomock.Any()).Return(true)
				Expect(cm.Start(ctx)).To(Succeed())
			})

			It("should fail if caches cannot be synced", func() {
				factory.EXPECT().NewClientSet(ctx, key).Return(cs, "", nil)
				Expect(cm.GetClient(ctx, key)).To(BeIdenticalTo(cs))

				cs.EXPECT().Start(gomock.Any())
				cs.EXPECT().WaitForCacheSync(gomock.Any()).Return(false)
				Expect(cm.Start(ctx)).To(MatchError(ContainSubstring("timed out waiting for caches of ClientSet")))
			})
		})
	})
})
