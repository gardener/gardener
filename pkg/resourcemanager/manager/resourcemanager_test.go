// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package manager

import (
	"context"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Resource Manager", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("Secrets", func() {
		ctx := context.TODO()

		It("should correctly create a managed secret", func() {
			var (
				secretName      = "foo"
				secretNamespace = "bar"
				secretLabels    = map[string]string{
					"boo": "goo",
				}
				secretAnnotations = map[string]string{
					"a": "b",
				}

				secretData = map[string][]byte{
					"foo": []byte("bar"),
				}

				secretMeta = metav1.ObjectMeta{
					Name:        secretName,
					Namespace:   secretNamespace,
					Annotations: secretAnnotations,
					Labels:      secretLabels,
				}
				expectedSecret = &corev1.Secret{
					ObjectMeta: secretMeta,
					Data:       secretData,
				}
			)

			managedSecret := NewSecret(c).
				WithNamespacedName(secretNamespace, secretName).
				WithKeyValues(secretData).
				WithLabels(secretLabels).
				WithAnnotations(secretAnnotations)
			Expect(managedSecret.secret).To(Equal(expectedSecret))

			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: secretNamespace, Name: secretName}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, secret *corev1.Secret) error {
				return apierrors.NewNotFound(corev1.Resource("secrets"), secretName)
			})

			expectedSecret.Type = corev1.SecretTypeOpaque
			c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, secret *corev1.Secret, _ ...client.CreateOption) error {
				Expect(secret).To(DeepEqual(expectedSecret))
				return nil
			})

			err := managedSecret.Reconcile(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should correctly create a managed resource", func() {
			var (
				managedResourceName      = "foo"
				managedResourceNamespace = "bar"
				managedResourceLabels    = map[string]string{
					"boo": "goo",
				}
				managedResourceAnnotations = map[string]string{
					"a": "b",
				}

				managedResourceMeta = metav1.ObjectMeta{
					Name:        managedResourceName,
					Namespace:   managedResourceNamespace,
					Labels:      managedResourceLabels,
					Annotations: managedResourceAnnotations,
				}

				resourceClass = "shoot"
				secretRefs    = []corev1.LocalObjectReference{
					{Name: "test1"},
					{Name: "test2"},
					{Name: "test3"},
				}

				injectedLabels = map[string]string{
					"shoot.gardener.cloud/no-cleanup": "true",
				}

				forceOverwriteAnnotations    = true
				forceOverwriteLabels         = true
				keepObjects                  = true
				deletePersistentVolumeClaims = true

				expectedManagedResource = &resourcesv1alpha1.ManagedResource{
					ObjectMeta: managedResourceMeta,
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						SecretRefs:                   secretRefs,
						InjectLabels:                 injectedLabels,
						Class:                        pointer.StringPtr(resourceClass),
						ForceOverwriteAnnotations:    pointer.BoolPtr(forceOverwriteAnnotations),
						ForceOverwriteLabels:         pointer.BoolPtr(forceOverwriteLabels),
						KeepObjects:                  pointer.BoolPtr(keepObjects),
						DeletePersistentVolumeClaims: pointer.BoolPtr(deletePersistentVolumeClaims),
					},
				}
			)

			managedResource := NewManagedResource(c).
				WithNamespacedName(managedResourceNamespace, managedResourceName).
				WithLabels(managedResourceLabels).
				WithAnnotations(managedResourceAnnotations).
				WithClass(resourceClass).
				WithSecretRef(secretRefs[0].Name).
				WithSecretRefs(secretRefs[1:]).
				WithInjectedLabels(injectedLabels).
				ForceOverwriteAnnotations(forceOverwriteAnnotations).
				ForceOverwriteLabels(forceOverwriteLabels).
				KeepObjects(keepObjects).
				DeletePersistentVolumeClaims(deletePersistentVolumeClaims)
			Expect(managedResource.resource).To(Equal(expectedManagedResource))

			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: managedResourceNamespace, Name: managedResourceName}, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, ms *resourcesv1alpha1.ManagedResource) error {
				return apierrors.NewNotFound(corev1.Resource("managedresources"), managedResourceName)
			})

			c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(func(_ context.Context, mr *resourcesv1alpha1.ManagedResource, _ ...client.CreateOption) error {
				Expect(mr).To(DeepEqual(expectedManagedResource))
				return nil
			})

			err := managedResource.Reconcile(ctx)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
