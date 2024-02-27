// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package backupentry_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/gardener/gardener/extensions/pkg/controller/backupentry"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	mockcache "github.com/gardener/gardener/pkg/mock/controller-runtime/cache"
	mockmanager "github.com/gardener/gardener/pkg/mock/controller-runtime/manager"
)

var _ = Describe("Controller Mapper", func() {
	var (
		ctx = context.TODO()

		fakeClient client.Client
		ctrl       *gomock.Controller
		cache      *mockcache.MockCache
		mgr        *mockmanager.MockManager

		namespace *corev1.Namespace
		configMap *corev1.ConfigMap
		secret    *corev1.Secret

		backupEntry  *extensionsv1alpha1.BackupEntry
		backupEntry2 *extensionsv1alpha1.BackupEntry
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		ctrl = gomock.NewController(GinkgoT())
		cache = mockcache.NewMockCache(ctrl)
		mgr = mockmanager.NewMockManager(ctrl)
		mgr.EXPECT().GetCache().Return(cache).AnyTimes()

		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-namespace",
				Annotations: map[string]string{
					v1beta1constants.ShootUID: "xyz",
				},
			},
		}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: namespace.Name,
			},
		}

		backupEntry = &extensionsv1alpha1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-namespace--xyz",
			},
			Spec: extensionsv1alpha1.BackupEntrySpec{
				SecretRef: corev1.SecretReference{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
			},
		}
		backupEntry2 = &extensionsv1alpha1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name: "backupEntry-2",
			},
			Spec: extensionsv1alpha1.BackupEntrySpec{
				SecretRef: corev1.SecretReference{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
			},
		}
	})

	Describe("#SecretToBackupEntryMapper", func() {
		var mapper mapper.Mapper

		BeforeEach(func() {
			mapper = SecretToBackupEntryMapper(nil)
		})

		It("should find all objects for the passed secret", func() {
			Expect(fakeClient.Create(ctx, backupEntry)).To(Succeed())
			Expect(fakeClient.Create(ctx, backupEntry2)).To(Succeed())

			Expect(mapper.Map(ctx, logr.Discard(), fakeClient, secret)).To(ConsistOf(
				reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: backupEntry.Name,
					},
				},
				reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: backupEntry2.Name,
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
			mapper = SecretToBackupEntryMapper(predicates)

			Expect(fakeClient.Create(ctx, backupEntry)).To(Succeed())
			Expect(mapper.Map(ctx, logr.Discard(), fakeClient, secret)).To(BeEmpty())
		})

		It("should return empty request array because there are no backupentry objects present", func() {
			Expect(mapper.Map(ctx, logr.Discard(), fakeClient, secret)).To(BeEmpty())
		})

		It("should find no objects because the passed object is not secret", func() {
			Expect(mapper.Map(ctx, logr.Discard(), fakeClient, configMap)).To(BeEmpty())
		})
	})

	Describe("#NamespaceToBackupEntryMapper", func() {
		var mapper mapper.Mapper

		BeforeEach(func() {
			mapper = NamespaceToBackupEntryMapper(nil)
		})

		It("should find all objects for the passed namespace", func() {
			Expect(fakeClient.Create(ctx, backupEntry)).To(Succeed())
			Expect(fakeClient.Create(ctx, backupEntry2)).To(Succeed())

			Expect(mapper.Map(ctx, logr.Discard(), fakeClient, namespace)).To(ConsistOf(
				reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: backupEntry.Name,
					},
				}))
		})

		It("should find no objects for the passed namespace because predicates do not match", func() {
			predicates := []predicate.Predicate{
				predicate.Funcs{
					GenericFunc: func(_ event.GenericEvent) bool {
						return false
					},
				},
			}
			mapper = NamespaceToBackupEntryMapper(predicates)

			Expect(fakeClient.Create(ctx, backupEntry)).To(Succeed())
			Expect(mapper.Map(ctx, logr.Discard(), fakeClient, namespace)).To(BeEmpty())
		})

		It("should return empty request array because there are no backupentry objects present", func() {
			Expect(mapper.Map(ctx, logr.Discard(), fakeClient, namespace)).To(BeEmpty())
		})

		It("should find no objects because the passed object is not namespace", func() {
			Expect(mapper.Map(ctx, logr.Discard(), fakeClient, configMap)).To(BeEmpty())
		})
	})
})
