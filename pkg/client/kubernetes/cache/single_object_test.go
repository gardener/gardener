// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cache_test

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/gardener/gardener/pkg/client/kubernetes/cache"
	mockcache "github.com/gardener/gardener/third_party/mock/controller-runtime/cache"
)

var _ = Describe("SingleObject", func() {
	var (
		parentCtx context.Context
		ctx       context.Context
		ctrl      *gomock.Controller

		obj        *corev1.Secret
		gvk        = corev1.SchemeGroupVersion.WithKind("Secret")
		key        client.ObjectKey
		syncPeriod = ptr.To(time.Second)

		mockCache         *mockcache.MockCache
		singleObjectCache cache.Cache
		clock             *testclock.FakeClock
		maxIdleTime       = 100 * time.Millisecond
	)

	BeforeEach(func() {
		ctx = context.Background()

		ctrl = gomock.NewController(GinkgoT())
		DeferCleanup(func() {
			ctrl.Finish()
		})

		obj = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
		}
		key = client.ObjectKeyFromObject(obj)

		mockCache = mockcache.NewMockCache(ctrl)
		newCacheFunc := func(_ *rest.Config, opts cache.Options) (cache.Cache, error) {
			Expect(opts.DefaultNamespaces).To(HaveKeyWithValue(obj.Namespace, cache.Config{}))
			Expect(opts.DefaultFieldSelector).To(Equal(fields.SelectorFromSet(fields.Set{"metadata.name": obj.Name})))
			Expect(opts.ByObject).To(BeNil())
			Expect(opts.SyncPeriod).To(Equal(syncPeriod))
			return mockCache, nil
		}
		clock = testclock.NewFakeClock(time.Now())

		singleObjectCache = NewSingleObject(logr.Discard(), nil, newCacheFunc, cache.Options{SyncPeriod: syncPeriod}, gvk, clock, maxIdleTime, 50*time.Millisecond)

		var cancel context.CancelFunc
		parentCtx, cancel = context.WithCancel(context.Background())

		go func() {
			defer GinkgoRecover()
			Expect(singleObjectCache.Start(parentCtx)).To(Succeed())
		}()

		DeferCleanup(func() {
			cancel()
		})

		Expect(singleObjectCache.WaitForCacheSync(parentCtx)).Should(BeTrue())
	})

	Describe("#Get", func() {
		It("should successfully delegate to stored cache", func() {
			testCache(parentCtx, mockCache, gvk, clock, maxIdleTime,
				func() {
					mockCache.EXPECT().Get(ctx, key, obj)
				},
				func() error {
					return singleObjectCache.Get(ctx, key, obj)
				},
			)
		})

		Describe("ensure caller context is not used for starting cache", func() {
			var (
				tmpCtx context.Context
				cancel context.CancelFunc
			)

			BeforeEach(func() {
				tmpCtx, cancel = context.WithCancel(ctx)
				DeferCleanup(func() { cancel() })
			})

			It("should not stop the cache when the context of the caller is canceled", func() {
				testCache(parentCtx, mockCache, gvk, clock, maxIdleTime,
					func() {
						mockCache.EXPECT().Get(tmpCtx, key, obj)
					},
					func() error {
						err := singleObjectCache.Get(tmpCtx, key, obj)
						cancel()
						return err
					},
				)
			})
		})
	})

	Describe("#GetInformer", func() {
		It("should delegate call to stored cache", func() {
			testCache(parentCtx, mockCache, gvk, clock, maxIdleTime,
				func() {
					mockCache.EXPECT().GetInformer(ctx, obj)
				},
				func() error {
					_, err := singleObjectCache.GetInformer(ctx, obj)
					return err
				},
			)
		})
	})

	Describe("#IndexField", func() {
		It("should delegate call to stored cache", func() {
			testCache(parentCtx, mockCache, gvk, clock, maxIdleTime,
				func() {
					mockCache.EXPECT().IndexField(ctx, obj, "", nil)
				},
				func() error {
					return singleObjectCache.IndexField(ctx, obj, "", nil)
				},
			)
		})
	})

	Describe("#List", func() {
		It("should not be implemented", func() {
			Expect(singleObjectCache.List(ctx, nil)).To(MatchError("the List operation is not supported by singleObject cache"))
		})
	})

	Describe("#GetInformerForKind", func() {
		It("should not be implemented", func() {
			_, err := singleObjectCache.GetInformerForKind(ctx, schema.GroupVersionKind{})
			Expect(err).To(MatchError("the GetInformerForKind operation is not supported by singleObject cache"))
		})
	})
})

func testCache(
	parentCtx context.Context,
	mockCache *mockcache.MockCache,
	gvk schema.GroupVersionKind,
	clock *testclock.FakeClock,
	maxIdleTime time.Duration,
	setupExpectation func(),
	testCall func() error,
) {
	var (
		cacheCtx  context.Context
		startChan = make(chan struct{})
	)

	waitForSyncCtx, waitForSyncCancel := context.WithTimeout(parentCtx, 5*time.Second)
	defer waitForSyncCancel()

	By("cache does not exist yet")
	mockCache.EXPECT().Start(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
		cacheCtx = ctx
		startChan <- struct{}{}
		return nil
	})
	mockCache.EXPECT().WaitForCacheSync(gomock.AssignableToTypeOf(waitForSyncCtx)).Return(true)
	mockCache.EXPECT().GetInformerForKind(gomock.AssignableToTypeOf(waitForSyncCtx), gvk)
	setupExpectation()
	ExpectWithOffset(1, testCall()).To(Succeed())
	EventuallyWithOffset(1, startChan).Should(Receive())

	By("cache is re-used")
	ConsistentlyWithOffset(1, func() <-chan struct{} {
		return cacheCtx.Done()
	}).ShouldNot(BeClosed())
	setupExpectation()
	ExpectWithOffset(1, testCall()).To(Succeed())

	By("cache expires")
	clock.Step(2 * maxIdleTime)
	EventuallyWithOffset(1, func() <-chan struct{} {
		return cacheCtx.Done()
	}).Should(BeClosed())

	By("cache is re-created")
	startChan2 := make(chan struct{})
	mockCache.EXPECT().Start(gomock.Any()).DoAndReturn(func(_ context.Context) error {
		startChan2 <- struct{}{}
		return nil
	})
	mockCache.EXPECT().WaitForCacheSync(gomock.AssignableToTypeOf(waitForSyncCtx)).Return(true)
	mockCache.EXPECT().GetInformerForKind(gomock.AssignableToTypeOf(waitForSyncCtx), gvk)
	setupExpectation()
	ExpectWithOffset(1, testCall()).To(Succeed())
	EventuallyWithOffset(1, startChan2).Should(Receive())

	By("ensure parent context is never closed")
	ConsistentlyWithOffset(1, func() <-chan struct{} {
		return parentCtx.Done()
	}).ShouldNot(BeClosed())
}
