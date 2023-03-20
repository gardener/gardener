// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package builder

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("Resource Manager", func() {
	var (
		ctx        = context.TODO()
		fakeClient client.Client
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
	})

	Context("Secrets", func() {
		var (
			name        = "foo"
			namespace   = "bar"
			labels      = map[string]string{"boo": "goo"}
			annotations = map[string]string{"a": "b"}
		)

		It("should correctly create a managed secret", func() {
			data := map[string][]byte{"foo": []byte("bar")}

			Expect(
				NewSecret(fakeClient).
					WithNamespacedName(namespace, name).
					WithKeyValues(data).
					WithLabels(labels).
					WithAnnotations(annotations).
					Reconcile(ctx),
			).To(Succeed())

			secret := &corev1.Secret{}
			Expect(fakeClient.Get(ctx, kubernetesutils.Key(namespace, name), secret)).To(Succeed())

			Expect(secret).To(Equal(&corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            name,
					Namespace:       namespace,
					Annotations:     annotations,
					Labels:          labels,
					ResourceVersion: "1",
				},
				Type: corev1.SecretTypeOpaque,
				Data: data,
			}))
		})

		It("should remove existing annotations or labels", func() {
			var (
				existingLabels      = map[string]string{"existing": "label"}
				existingAnnotations = map[string]string{"existing": "annotation"}
			)

			mr := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        name,
					Namespace:   namespace,
					Labels:      existingLabels,
					Annotations: existingAnnotations,
				},
			}
			Expect(fakeClient.Create(ctx, mr)).To(Succeed())

			Expect(
				NewSecret(fakeClient).
					WithNamespacedName(namespace, name).
					WithLabels(labels).
					WithAnnotations(annotations).
					Reconcile(ctx),
			).To(Succeed())

			Expect(fakeClient.Get(ctx, kubernetesutils.Key(namespace, name), mr)).To(Succeed())

			Expect(mr).To(Equal(&corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            name,
					Namespace:       namespace,
					Labels:          labels,
					Annotations:     annotations,
					ResourceVersion: "2",
				},
				Type: corev1.SecretTypeOpaque,
			}))
		})
	})

	Context("ManagedResources", func() {
		var (
			name        = "foo"
			namespace   = "bar"
			labels      = map[string]string{"boo": "goo"}
			annotations = map[string]string{"a": "b"}
		)

		It("should correctly create a managed resource", func() {
			var (
				resourceClass = "shoot"
				secretRefs    = []corev1.LocalObjectReference{
					{Name: "test1"},
					{Name: "test2"},
					{Name: "test3"},
				}

				injectedLabels = map[string]string{"shoot.gardener.cloud/no-cleanup": "true"}

				forceOverwriteAnnotations    = true
				forceOverwriteLabels         = true
				keepObjects                  = true
				deletePersistentVolumeClaims = true
			)

			Expect(
				NewManagedResource(fakeClient).
					WithNamespacedName(namespace, name).
					WithLabels(labels).
					WithAnnotations(annotations).
					WithClass(resourceClass).
					WithSecretRef(secretRefs[0].Name).
					WithSecretRefs(secretRefs[1:]).
					WithInjectedLabels(injectedLabels).
					ForceOverwriteAnnotations(forceOverwriteAnnotations).
					ForceOverwriteLabels(forceOverwriteLabels).
					KeepObjects(keepObjects).
					DeletePersistentVolumeClaims(deletePersistentVolumeClaims).
					Reconcile(ctx),
			).To(Succeed())

			mr := &resourcesv1alpha1.ManagedResource{}
			Expect(fakeClient.Get(ctx, kubernetesutils.Key(namespace, name), mr)).To(Succeed())

			Expect(mr).To(Equal(&resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "resources.gardener.cloud/v1alpha1",
					Kind:       "ManagedResource",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            name,
					Namespace:       namespace,
					Labels:          labels,
					Annotations:     annotations,
					ResourceVersion: "1",
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					SecretRefs:                   secretRefs,
					InjectLabels:                 injectedLabels,
					Class:                        pointer.String(resourceClass),
					ForceOverwriteAnnotations:    pointer.Bool(forceOverwriteAnnotations),
					ForceOverwriteLabels:         pointer.Bool(forceOverwriteLabels),
					KeepObjects:                  pointer.Bool(keepObjects),
					DeletePersistentVolumeClaims: pointer.Bool(deletePersistentVolumeClaims),
				},
			}))
		})

		It("should keep existing annotations or labels", func() {
			var (
				existingLabels      = map[string]string{"existing": "label"}
				existingAnnotations = map[string]string{"existing": "annotation"}
			)

			mr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:        name,
					Namespace:   namespace,
					Labels:      existingLabels,
					Annotations: existingAnnotations,
				},
			}
			Expect(fakeClient.Create(ctx, mr)).To(Succeed())

			Expect(
				NewManagedResource(fakeClient).
					WithNamespacedName(namespace, name).
					WithLabels(labels).
					WithAnnotations(annotations).
					Reconcile(ctx),
			).To(Succeed())

			Expect(fakeClient.Get(ctx, kubernetesutils.Key(namespace, name), mr)).To(Succeed())

			Expect(mr).To(Equal(&resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "resources.gardener.cloud/v1alpha1",
					Kind:       "ManagedResource",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            name,
					Namespace:       namespace,
					Labels:          utils.MergeStringMaps(existingLabels, labels),
					Annotations:     utils.MergeStringMaps(existingAnnotations, annotations),
					ResourceVersion: "2",
				},
			}))
		})
	})
})
