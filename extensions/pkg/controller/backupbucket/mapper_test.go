// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockcache "github.com/gardener/gardener/third_party/mock/controller-runtime/cache"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
)

var _ = Describe("Controller Mapper", func() {
	var (
		ctx = context.TODO()

		fakeClient client.Client
		ctrl       *gomock.Controller
		cache      *mockcache.MockCache
		mgr        *mockmanager.MockManager

		namespace = "some-namespace"
		configMap *corev1.ConfigMap
		secret    *corev1.Secret

		backupBucket  *extensionsv1alpha1.BackupBucket
		backupBucket2 *extensionsv1alpha1.BackupBucket
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		ctrl = gomock.NewController(GinkgoT())
		cache = mockcache.NewMockCache(ctrl)
		mgr = mockmanager.NewMockManager(ctrl)
		mgr.EXPECT().GetCache().Return(cache).AnyTimes()

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: namespace,
			},
		}

		backupBucket = &extensionsv1alpha1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: "backupBucket-1",
			},
			Spec: extensionsv1alpha1.BackupBucketSpec{
				SecretRef: corev1.SecretReference{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
			},
		}
		backupBucket2 = &extensionsv1alpha1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: "backupBucket-2",
			},
			Spec: extensionsv1alpha1.BackupBucketSpec{
				SecretRef: corev1.SecretReference{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
			},
		}
	})

	Describe("#SecretToBackupBucketMapper", func() {
		var mapper handler.MapFunc

		BeforeEach(func() {
			mapper = SecretToBackupBucketMapper(fakeClient, nil)
		})

		It("should find all objects for the passed secret", func() {
			Expect(fakeClient.Create(ctx, backupBucket)).To(Succeed())
			Expect(fakeClient.Create(ctx, backupBucket2)).To(Succeed())

			Expect(mapper(ctx, secret)).To(ConsistOf(
				reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: backupBucket.Name,
					},
				},
				reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: backupBucket2.Name,
					},
				}))
		})

		It("should find no objects for the passed secret because predicates do not match", func() {
			predicates := []predicate.Predicate{
				predicate.Funcs{
					GenericFunc: func(_ event.GenericEvent) bool {
						return false
					},
				},
			}
			mapper = SecretToBackupBucketMapper(fakeClient, predicates)

			Expect(fakeClient.Create(ctx, backupBucket)).To(Succeed())
			Expect(mapper(ctx, secret)).To(BeEmpty())
		})

		It("should return empty request array because there are no backupbucket objects present", func() {
			Expect(mapper(ctx, secret)).To(BeEmpty())
		})

		It("should find no objects because the passed object is not a secret", func() {
			Expect(mapper(ctx, configMap)).To(BeEmpty())
		})
	})
})
