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

package common_test

import (
	"context"
	"fmt"

	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	resourcesv1alpha1 "github.com/gardener/gardener-resource-manager/pkg/apis/resources/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var _ = Describe("ManagedResources", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		ctx         = context.TODO()
		fakeErr     = fmt.Errorf("fake err")
		name        = "foo"
		namespace   = "bar"
		keepObjects = true
		data        = map[string][]byte{"some": []byte("data")}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeployManagedResourceForShoot", func() {
		It("should return the error of the secret reconciliation", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, kutil.Key(namespace, "managedresource-"+name), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr),
			)

			Expect(DeployManagedResourceForShoot(ctx, c, name, namespace, keepObjects, data)).To(MatchError(fakeErr))
		})

		It("should return the error of the managed resource reconciliation", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, kutil.Key(namespace, "managedresource-"+name), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr),
			)

			Expect(DeployManagedResourceForShoot(ctx, c, name, namespace, keepObjects, data)).To(MatchError(fakeErr))
		})

		It("should successfully create secret and managed resource", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, kutil.Key(namespace, "managedresource-"+name), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "managedresource-" + name,
						Namespace: namespace,
					},
					Type: corev1.SecretTypeOpaque,
					Data: data,
				}),
				c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				c.EXPECT().Update(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
						Labels:    map[string]string{"origin": "gardener"},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						SecretRefs:   []corev1.LocalObjectReference{{Name: "managedresource-" + name}},
						KeepObjects:  pointer.BoolPtr(keepObjects),
						InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
					},
				}),
			)

			Expect(DeployManagedResourceForShoot(ctx, c, name, namespace, keepObjects, data)).To(Succeed())
		})
	})

	Describe("#DeleteManagedResourceForShoot", func() {
		var (
			secret          = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "managedresource-" + name}}
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}
		)

		It("should fail when the managed resource cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, managedResource).Return(fakeErr),
			)

			Expect(DeleteManagedResourceForShoot(ctx, c, name, namespace)).To(MatchError(fakeErr))
		})

		It("should fail when the secret cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, managedResource),
				c.EXPECT().Delete(ctx, secret).Return(fakeErr),
			)

			Expect(DeleteManagedResourceForShoot(ctx, c, name, namespace)).To(MatchError(fakeErr))
		})

		It("should successfully delete all related resources", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, managedResource),
				c.EXPECT().Delete(ctx, secret),
			)

			Expect(DeleteManagedResourceForShoot(ctx, c, name, namespace)).To(Succeed())
		})
	})

	Describe("#DeployManagedResourceForSeed", func() {
		It("should return the error of the secret reconciliation", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, kutil.Key(namespace, "managedresource-"+name), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr),
			)

			Expect(DeployManagedResourceForSeed(ctx, c, name, namespace, keepObjects, data)).To(MatchError(fakeErr))
		})

		It("should return the error of the managed resource reconciliation", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, kutil.Key(namespace, "managedresource-"+name), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr),
			)

			Expect(DeployManagedResourceForSeed(ctx, c, name, namespace, keepObjects, data)).To(MatchError(fakeErr))
		})

		It("should successfully create secret and managed resource", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, kutil.Key(namespace, "managedresource-"+name), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "managedresource-" + name,
						Namespace: namespace,
					},
					Type: corev1.SecretTypeOpaque,
					Data: data,
				}),
				c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				c.EXPECT().Update(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						SecretRefs:  []corev1.LocalObjectReference{{Name: "managedresource-" + name}},
						KeepObjects: pointer.BoolPtr(keepObjects),
						Class:       pointer.StringPtr("seed"),
					},
				}),
			)

			Expect(DeployManagedResourceForSeed(ctx, c, name, namespace, keepObjects, data)).To(Succeed())
		})
	})

	Describe("#DeleteManagedResourceForSeed", func() {
		var (
			secret          = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "managedresource-" + name}}
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}
		)

		It("should fail when the managed resource cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, managedResource).Return(fakeErr),
			)

			Expect(DeleteManagedResourceForSeed(ctx, c, name, namespace)).To(MatchError(fakeErr))
		})

		It("should fail when the secret cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, managedResource),
				c.EXPECT().Delete(ctx, secret).Return(fakeErr),
			)

			Expect(DeleteManagedResourceForSeed(ctx, c, name, namespace)).To(MatchError(fakeErr))
		})

		It("should successfully delete all related resources", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, managedResource),
				c.EXPECT().Delete(ctx, secret),
			)

			Expect(DeleteManagedResourceForSeed(ctx, c, name, namespace)).To(Succeed())
		})
	})
})
