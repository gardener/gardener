// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

	Describe("#DeployManagedResource", func() {
		It("should return the error of the secret reconciliation", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, kutil.Key(namespace, "managedresource-"+name), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr),
			)

			Expect(DeployManagedResource(ctx, c, name, namespace, keepObjects, data)).To(MatchError(fakeErr))
		})

		It("should return the error of the managed resource reconciliation", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, kutil.Key(namespace, "managedresource-"+name), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr),
			)

			Expect(DeployManagedResource(ctx, c, name, namespace, keepObjects, data)).To(MatchError(fakeErr))
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

			Expect(DeployManagedResource(ctx, c, name, namespace, keepObjects, data)).To(Succeed())
		})
	})
})
