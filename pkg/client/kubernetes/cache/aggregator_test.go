// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cache_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/client/kubernetes/cache"
	mockcache "github.com/gardener/gardener/third_party/mock/controller-runtime/cache"
)

var _ = Describe("Aggregator", func() {
	var (
		ctx         context.Context
		ctrl        *gomock.Controller
		fallback    *mockcache.MockCache
		secretCache *mockcache.MockCache
		gvkToCache  map[schema.GroupVersionKind]cache.Cache
		scheme      *runtime.Scheme

		aggregator cache.Cache
	)

	BeforeEach(func() {
		ctx = context.Background()
		ctrl = gomock.NewController(GinkgoT())
		fallback = mockcache.NewMockCache(ctrl)
		secretCache = mockcache.NewMockCache(ctrl)
		gvkToCache = map[schema.GroupVersionKind]cache.Cache{
			corev1.SchemeGroupVersion.WithKind("Secret"): secretCache,
		}
		scheme = runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())

		aggregator = NewAggregator(fallback, gvkToCache, scheme)
	})

	Describe("#Get", func() {
		var objectMeta metav1.ObjectMeta

		BeforeEach(func() {
			objectMeta = metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
			}
		})

		It("should get Secret from special cache", func() {
			secret := &corev1.Secret{
				ObjectMeta: objectMeta,
			}
			secretCache.EXPECT().Get(ctx, client.ObjectKeyFromObject(secret), secret).Return(nil)
			Expect(aggregator.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
		})

		It("should get ConfigMap from fallback cache", func() {
			cm := &corev1.ConfigMap{
				ObjectMeta: objectMeta,
			}
			fallback.EXPECT().Get(ctx, client.ObjectKeyFromObject(cm), cm).Return(nil)
			Expect(aggregator.Get(ctx, client.ObjectKeyFromObject(cm), cm)).To(Succeed())
		})

		It("should get Shoot from fallback cache", func() {
			shoot := &gardencorev1beta1.Shoot{
				ObjectMeta: objectMeta,
			}
			fallback.EXPECT().Get(ctx, client.ObjectKeyFromObject(shoot), shoot).Return(nil)
			Expect(aggregator.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
		})

		It("should return an error", func() {
			shoot := &gardencorev1beta1.Shoot{
				ObjectMeta: objectMeta,
			}
			fallback.EXPECT().Get(ctx, client.ObjectKeyFromObject(shoot), shoot).Return(errors.New("some error"))
			Expect(aggregator.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(MatchError("some error"))
		})
	})

	Describe("#List", func() {
		It("should list Secrets from special cache", func() {
			secrets := &corev1.SecretList{}
			secretCache.EXPECT().List(ctx, secrets).Return(nil)
			Expect(aggregator.List(ctx, secrets)).To(Succeed())
		})

		It("should list ConfigMaps from fallback cache", func() {
			cms := &corev1.ConfigMapList{}
			fallback.EXPECT().List(ctx, cms).Return(nil)
			Expect(aggregator.List(ctx, cms)).To(Succeed())
		})

		It("should list Shoots from fallback cache", func() {
			shoots := &gardencorev1beta1.ShootList{}
			fallback.EXPECT().List(ctx, shoots).Return(nil)
			Expect(aggregator.List(ctx, shoots)).To(Succeed())
		})
	})

	Describe("#GetInformer", func() {
		It("should get informer for Secret from special cache", func() {
			secret := &corev1.Secret{}
			secretCache.EXPECT().GetInformer(ctx, secret)
			_, err := aggregator.GetInformer(ctx, secret)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should get informer for ConfigMap from fallback cache", func() {
			cm := &corev1.ConfigMap{}
			fallback.EXPECT().GetInformer(ctx, cm)
			_, err := aggregator.GetInformer(ctx, cm)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#GetInformerForKind", func() {
		It("should get informer for Secret from special cache", func() {
			gvk := corev1.SchemeGroupVersion.WithKind("Secret")
			secretCache.EXPECT().GetInformerForKind(ctx, gvk)
			_, err := aggregator.GetInformerForKind(ctx, gvk)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should get informer for SecretList from special cache", func() {
			gvk := corev1.SchemeGroupVersion.WithKind("SecretList")
			secretCache.EXPECT().GetInformerForKind(ctx, gvk)
			_, err := aggregator.GetInformerForKind(ctx, gvk)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should get informer for ConfigMap from fallback cache", func() {
			gvk := corev1.SchemeGroupVersion.WithKind("ConfigMap")
			fallback.EXPECT().GetInformerForKind(ctx, gvk)
			_, err := aggregator.GetInformerForKind(ctx, gvk)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#Start", func() {
		It("should run all informers until context is cancelled", func() {
			ctx, cancel := context.WithCancel(ctx)
			testChan := make(chan struct{})

			fallback.EXPECT().Start(ctx).DoAndReturn(func(_ context.Context) error {
				testChan <- struct{}{}
				return nil
			})
			secretCache.EXPECT().Start(ctx).DoAndReturn(func(_ context.Context) error {
				testChan <- struct{}{}
				return nil
			})

			var wg wait.Group
			wg.Start(func() {
				defer GinkgoRecover()
				Expect(aggregator.Start(ctx)).To(Succeed())
			})

			Eventually(testChan).Should(Receive())
			Eventually(testChan).Should(Receive())
			close(testChan)
			// cancel ctx to stop aggregator cache
			cancel()
			// wait for aggregator.Start to return, otherwise test is not finished
			wg.Wait()
		})
	})

	Describe("#WaitForCacheSync", func() {
		It("should return true because all caches are synced", func() {
			fallback.EXPECT().WaitForCacheSync(ctx).Return(true)
			secretCache.EXPECT().WaitForCacheSync(ctx).Return(true)
			Expect(aggregator.WaitForCacheSync(ctx)).To(BeTrue())
		})

		It("should return false because Secret cache is not synced", func() {
			secretCache.EXPECT().WaitForCacheSync(ctx).Return(false)
			Expect(aggregator.WaitForCacheSync(ctx)).To(BeFalse())
		})

		It("should return false because fallback cache is not synced", func() {
			fallback.EXPECT().WaitForCacheSync(ctx).Return(false)
			secretCache.EXPECT().WaitForCacheSync(ctx).Return(true)
			Expect(aggregator.WaitForCacheSync(ctx)).To(BeFalse())
		})
	})

	Describe("#IndexField", func() {
		var indexerFunc client.IndexerFunc

		BeforeEach(func() {
			indexerFunc = func(_ client.Object) []string { return nil }
		})

		It("should call IndexField on Secret cache", func() {
			secret := &corev1.Secret{}
			secretCache.EXPECT().IndexField(ctx, secret, "foo", gomock.Any()).Return(nil)
			Expect(aggregator.IndexField(ctx, secret, "foo", indexerFunc)).To(Succeed())
		})

		It("should call IndexField on ConfigMap cache", func() {
			cm := &corev1.ConfigMap{}
			fallback.EXPECT().IndexField(ctx, cm, "foo", gomock.Any()).Return(nil)
			Expect(aggregator.IndexField(ctx, cm, "foo", indexerFunc)).To(Succeed())
		})
	})

	AfterEach(func() {
		ctrl.Finish()
	})
})
