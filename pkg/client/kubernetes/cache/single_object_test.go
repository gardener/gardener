// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package cache_test

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/gardener/gardener/pkg/client/kubernetes/cache"
	mockcache "github.com/gardener/gardener/pkg/mock/controller-runtime/cache"
)

var _ = Describe("SingleObject", func() {
	var (
		ctx  context.Context
		ctrl *gomock.Controller

		obj    *corev1.Secret
		key    client.ObjectKey
		resync = pointer.Duration(time.Second)

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
			Expect(opts.Namespace).To(Equal(obj.Namespace))
			Expect(opts.DefaultSelector).To(Equal(cache.ObjectSelector{Field: fields.SelectorFromSet(fields.Set{"metadata.name": obj.Name})}))
			Expect(opts.SelectorsByObject).To(BeNil())
			Expect(opts.Resync).To(Equal(resync))
			return mockCache, nil
		}
		clock = testclock.NewFakeClock(time.Now())

		singleObjectCache = NewSingleObject(logr.Discard(), nil, newCacheFunc, cache.Options{Resync: resync}, clock, maxIdleTime, 50*time.Millisecond)

		var wg wait.Group
		ctx, cancel := context.WithCancel(ctx)

		DeferCleanup(func() {
			// cancel ctx to stop garbage collector
			cancel()
			// wait for singleObjectCache.Start to return, otherwise test is not finished
			wg.Wait()
		})

		wg.Start(func() {
			defer GinkgoRecover()
			Expect(singleObjectCache.Start(ctx)).To(Succeed())
		})
	})

	Describe("#Get", func() {
		It("should successfully delegate to stored cache", func() {
			testCache(ctx, mockCache, clock, maxIdleTime,
				func() {
					mockCache.EXPECT().Get(ctx, key, obj)
				},
				func() error {
					return singleObjectCache.Get(ctx, key, obj)
				})
		})
	})

	Describe("#GetInformer", func() {
		It("should delegate call to stored cache", func() {
			testCache(ctx, mockCache, clock, maxIdleTime,
				func() {
					mockCache.EXPECT().GetInformer(ctx, obj)
				},
				func() error {
					_, err := singleObjectCache.GetInformer(ctx, obj)
					return err
				})
		})
	})

	Describe("#IndexField", func() {
		It("should delegate call to stored cache", func() {
			testCache(ctx, mockCache, clock, maxIdleTime,
				func() {
					mockCache.EXPECT().IndexField(ctx, obj, "", nil)
				},
				func() error {
					return singleObjectCache.IndexField(ctx, obj, "", nil)
				})
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

func expectStart(mockCache *mockcache.MockCache) <-chan struct{} {
	testChan := make(chan struct{})
	mockCache.EXPECT().Start(gomock.Any()).DoAndReturn(func(_ context.Context) error {
		testChan <- struct{}{}
		return nil
	})

	return testChan
}

func testCache(ctx context.Context, mockCache *mockcache.MockCache, clock *testclock.FakeClock, maxIdleTime time.Duration, setupExpectation func(), testCall func() error) {
	By("cache does not exist yet")
	var (
		cacheCtx  context.Context
		startChan = make(chan struct{})
	)

	mockCache.EXPECT().Start(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
		cacheCtx = ctx
		startChan <- struct{}{}
		return nil
	})

	mockCache.EXPECT().WaitForCacheSync(ctx).Return(true)
	setupExpectation()
	ExpectWithOffset(1, testCall()).To(Succeed())

	By("ensure mock cache was started")
	// The `Start()` call of the cache is done as part of a go function, hence it can race with this test.
	// Before making assertions on `cacheCtx` we should wait for it to have been set.
	EventuallyWithOffset(1, startChan).Should(Receive())

	By("cache is re-used")
	setupExpectation()
	ExpectWithOffset(1, testCall()).To(Succeed())

	By("cache expires")
	clock.Step(2 * maxIdleTime)
	EventuallyWithOffset(1, func() <-chan struct{} {
		return cacheCtx.Done()
	}).Should(BeClosed())

	By("cache is re-created")
	startChan2 := expectStart(mockCache)
	mockCache.EXPECT().WaitForCacheSync(ctx).Return(true)
	setupExpectation()
	ExpectWithOffset(1, testCall()).To(Succeed())
	EventuallyWithOffset(1, startChan2).Should(Receive())
}
