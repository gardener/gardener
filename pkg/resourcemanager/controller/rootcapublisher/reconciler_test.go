// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package rootcapublisher_test

import (
	"context"
	"fmt"

	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/resourcemanager/controller/rootcapublisher"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	kubernetescheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Reconciler", func() {
	var (
		ctx = context.Background()

		fakeClient client.Client
		ctrl       reconcile.Reconciler
		request    reconcile.Request

		namespace *corev1.Namespace
		configMap *corev1.ConfigMap

		rootCA = "foo-bar"
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().
			WithScheme(kubernetescheme.Scheme).
			Build()

		ctrl = NewReconciler(fakeClient, rootCA)

		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "some-namespace",
			},
		}
		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace.Name,
				Name:      "kube-root-ca.crt",
			},
		}

		request = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name: namespace.Name,
			},
		}
	})

	Describe("#Reconcile", func() {
		It("should do nothing if namespace is gone", func() {
			result, err := ctrl.Reconcile(ctx, request)
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("namespace exists", func() {
			BeforeEach(func() {
				Expect(fakeClient.Create(ctx, namespace)).To(Succeed())
			})

			Context("create or update", func() {
				AfterEach(func() {
					result, err := ctrl.Reconcile(ctx, request)
					Expect(result).To(Equal(reconcile.Result{}))
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
					Expect(configMap.Data["ca.crt"]).To(Equal(rootCA))
					Expect(configMap.OwnerReferences).To(ConsistOf(metav1.OwnerReference{
						APIVersion:         "v1",
						Kind:               "Namespace",
						Name:               namespace.Name,
						BlockOwnerDeletion: pointer.Bool(false),
						Controller:         pointer.Bool(true),
					}))
				})

				It("should create a config map", func() {})

				It("should update the config map", func() {
					configMap.Data = map[string]string{"ca.crt": "bla"}
					configMap.OwnerReferences = nil
					Expect(fakeClient.Create(ctx, configMap)).To(Succeed())
				})
			})

			Context("error cases", func() {
				var (
					mockCtrl   *gomock.Controller
					mockClient *mockclient.MockClient

					fakeErr = fmt.Errorf("fake")
				)

				BeforeEach(func() {
					mockCtrl = gomock.NewController(GinkgoT())
					mockClient = mockclient.NewMockClient(mockCtrl)

					ctrl = NewReconciler(mockClient, rootCA)
				})

				AfterEach(func() {
					mockCtrl.Finish()
				})

				It("should fail because the namespace cannot be read", func() {
					mockClient.EXPECT().Get(ctx, request.NamespacedName, gomock.AssignableToTypeOf(&corev1.Namespace{})).Return(fakeErr)

					result, err := ctrl.Reconcile(ctx, request)
					Expect(result).To(Equal(reconcile.Result{}))
					Expect(err).To(MatchError(ContainSubstring(fakeErr.Error())))
				})

				It("should ignore 'not found' errors", func() {
					mockClient.EXPECT().Get(ctx, request.NamespacedName, gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Namespace) error {
						namespace.DeepCopyInto(obj)
						return nil
					})

					mockClient.EXPECT().Get(ctx, client.ObjectKeyFromObject(configMap), gomock.AssignableToTypeOf(&corev1.ConfigMap{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))
					mockClient.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

					result, err := ctrl.Reconcile(ctx, request)
					Expect(result).To(Equal(reconcile.Result{}))
					Expect(err).NotTo(HaveOccurred())
				})

				It("should ignore 'namespace terminating' errors", func() {
					mockClient.EXPECT().Get(ctx, request.NamespacedName, gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Namespace) error {
						namespace.DeepCopyInto(obj)
						return nil
					})

					mockClient.EXPECT().Get(ctx, client.ObjectKeyFromObject(configMap), gomock.AssignableToTypeOf(&corev1.ConfigMap{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

					mockClient.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).Return(&apierrors.StatusError{ErrStatus: metav1.Status{Details: &metav1.StatusDetails{Causes: []metav1.StatusCause{{Type: "NamespaceTerminating"}}}}})

					result, err := ctrl.Reconcile(ctx, request)
					Expect(result).To(Equal(reconcile.Result{}))
					Expect(err).NotTo(HaveOccurred())
				})
			})

			It("should not interfere with k8s publisher", func() {
				const rootCAManagedbyKCM = "bla"

				configMap.Data = map[string]string{"ca.crt": rootCAManagedbyKCM}
				configMap.Annotations = map[string]string{"kubernetes.io/description": "some big description"}
				Expect(fakeClient.Create(ctx, configMap)).To(Succeed())

				result, err := ctrl.Reconcile(ctx, request)
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap)).To(Succeed())
				Expect(configMap.Data["ca.crt"]).To(Equal(rootCAManagedbyKCM))
				Expect(configMap.OwnerReferences).To(BeEmpty())
			})
		})
	})
})
