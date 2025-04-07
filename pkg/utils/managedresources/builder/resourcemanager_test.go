// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package builder_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/managedresources/builder"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Resource Manager", func() {
	var (
		ctx        = context.Background()
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
					AddLabels(map[string]string{"one": "two"}).
					Reconcile(ctx),
			).To(Succeed())

			secret := &corev1.Secret{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret)).To(Succeed())

			Expect(secret).To(Equal(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        name,
					Namespace:   namespace,
					Annotations: annotations,
					Labels: map[string]string{
						"boo": "goo",
						"one": "two",
					},
					ResourceVersion: "1",
				},
				Type: corev1.SecretTypeOpaque,
				Data: data,
			}))
		})

		It("should correctly create an unique managed secret", func() {
			data := map[string][]byte{"foo": []byte("bar")}
			uniqueSecretName, secretBuilder := NewSecret(fakeClient).
				WithNamespacedName(namespace, name).
				WithKeyValues(data).
				WithLabels(labels).
				WithAnnotations(annotations).
				CreateIfNotExists(false).
				Unique()

			secretBuilder.AddLabels(map[string]string{"one": "two"})

			Expect(secretBuilder.Reconcile(ctx)).To(Succeed())

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      uniqueSecretName,
					Namespace: namespace,
				},
			}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())

			Expect(secret).To(Equal(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        uniqueSecretName,
					Namespace:   namespace,
					Annotations: annotations,
					Labels: map[string]string{
						"boo": "goo",
						"resources.gardener.cloud/garbage-collectable-reference": "true",
						"one": "two",
					},
					ResourceVersion: "1",
				},
				Type:      corev1.SecretTypeOpaque,
				Data:      data,
				Immutable: ptr.To(true),
			}))
		})

		It("should remove existing annotations or labels", func() {
			var (
				existingLabels      = map[string]string{"existing": "label"}
				existingAnnotations = map[string]string{"existing": "annotation"}
			)

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        name,
					Namespace:   namespace,
					Labels:      existingLabels,
					Annotations: existingAnnotations,
				},
			}
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			Expect(
				NewSecret(fakeClient).
					WithNamespacedName(namespace, name).
					WithLabels(labels).
					WithAnnotations(annotations).
					Reconcile(ctx),
			).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret)).To(Succeed())

			Expect(secret).To(Equal(&corev1.Secret{
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

		It("should not overwrite the GC label for immutable secrets", func() {
			secretName, secret := NewSecret(fakeClient).
				WithNamespacedName(namespace, name).
				WithLabels(labels).
				Unique()

			secret.WithLabels(map[string]string{"one": "two"})
			Expect(secret.Reconcile(ctx)).To(Succeed())

			actualSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(actualSecret), actualSecret)).To(Succeed())

			Expect(actualSecret).To(Equal(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespace,
					Labels: map[string]string{
						"one": "two",
						"resources.gardener.cloud/garbage-collectable-reference": "true",
					},
					ResourceVersion: "1",
				},
				Type:      corev1.SecretTypeOpaque,
				Immutable: ptr.To(true),
			}))
		})

		It("should update the secret if it exists", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Data: map[string][]byte{
					"foo": []byte("bar"),
				},
			}
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			Expect(
				NewSecret(fakeClient).
					WithNamespacedName(namespace, name).
					WithKeyValues(map[string][]byte{
						"bar": []byte("foo"),
					}).
					Reconcile(ctx),
			).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret)).To(Succeed())

			Expect(secret).To(Equal(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            name,
					Namespace:       namespace,
					ResourceVersion: "2",
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"bar": []byte("foo"),
				},
			}))
		})

		It("should fail to update the secret if it doesn't exist", func() {
			secret := NewSecret(fakeClient).
				WithNamespacedName(namespace, name).
				WithLabels(labels).
				CreateIfNotExists(false)

			Expect(secret.Reconcile(ctx)).To(BeNotFoundError())

			Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &corev1.Secret{})).To(BeNotFoundError())
		})
	})

	Context("ManagedResources", func() {
		var (
			name                      = "foo"
			namespace                 = "bar"
			labels                    = map[string]string{"boo": "goo"}
			annotations               = map[string]string{"a": "b"}
			secret1, secret2, secret3 *corev1.Secret
		)
		BeforeEach(func() {
			secret1 = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test1",
					Namespace:   namespace,
					Labels:      map[string]string{"foo": "bar"},
					Annotations: map[string]string{"foo": "bar"},
				},
				Data: map[string][]byte{"test": []byte("123")},
			}
			secret2 = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test2",
					Namespace: namespace,
					Labels:    map[string]string{"abc": "def"},
				},
				Data: map[string][]byte{"test": []byte("123")},
			}
			secret3 = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test3",
					Namespace: namespace,
					Labels:    map[string]string{},
				},
				Data: map[string][]byte{"test": []byte("123")},
			}
		})

		It("should correctly create a managed resource", func() {
			var (
				resourceClass = "shoot"
				secretRefs    = []corev1.LocalObjectReference{
					{Name: secret1.Name},
					{Name: secret2.Name},
					{Name: secret3.Name},
				}

				injectedLabels = map[string]string{"shoot.gardener.cloud/no-cleanup": "true"}

				forceOverwriteAnnotations    = true
				forceOverwriteLabels         = true
				keepObjects                  = true
				deletePersistentVolumeClaims = true
			)

			secrets := []*corev1.Secret{secret1, secret2, secret3}
			for _, s := range secrets {
				Expect(fakeClient.Create(ctx, s)).To(Succeed())
			}

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
			Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, mr)).To(Succeed())

			expectedMr := &resourcesv1alpha1.ManagedResource{
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
					Class:                        ptr.To(resourceClass),
					ForceOverwriteAnnotations:    ptr.To(forceOverwriteAnnotations),
					ForceOverwriteLabels:         ptr.To(forceOverwriteLabels),
					KeepObjects:                  ptr.To(keepObjects),
					DeletePersistentVolumeClaims: ptr.To(deletePersistentVolumeClaims),
				},
			}

			Expect(references.InjectAnnotations(expectedMr)).To(Succeed())
			Expect(mr).To(Equal(expectedMr))
		})

		It("should label existing managed resource secrets", func() {
			mr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:        name,
					Namespace:   namespace,
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					SecretRefs: []corev1.LocalObjectReference{
						{Name: secret1.Name},
						{Name: secret2.Name},
						{Name: secret3.Name},
					},
				},
			}

			// Create secrets without GC label
			Expect(fakeClient.Create(ctx, mr)).To(Succeed())
			for _, s := range []*corev1.Secret{secret1, secret2, secret3} {
				Expect(fakeClient.Create(ctx, s)).To(Succeed())
			}

			Expect(
				NewManagedResource(fakeClient).
					WithNamespacedName(namespace, name).
					WithLabels(labels).
					WithAnnotations(annotations).
					WithSecretRef(secret1.Name).
					WithSecretRef(secret2.Name).
					Reconcile(ctx),
			).To(Succeed())

			// Old and new secrets should be marked as garbage-collectable
			for _, s := range []*corev1.Secret{secret1, secret2, secret3} {
				secret := &corev1.Secret{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(s), secret)).To(Succeed())
				expected := s.DeepCopy()
				expected.ResourceVersion = "2"
				expected.Labels["resources.gardener.cloud/garbage-collectable-reference"] = "true"
				Expect(secret).To(Equal(expected))
			}
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

			Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, mr)).To(Succeed())

			Expect(mr).To(Equal(&resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:            name,
					Namespace:       namespace,
					Labels:          utils.MergeStringMaps(existingLabels, labels),
					Annotations:     utils.MergeStringMaps(existingAnnotations, annotations),
					ResourceVersion: "2",
				},
			}))
		})

		It("should update the managed resource if it exists", func() {
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
					CreateIfNotExists(false).
					Reconcile(ctx),
			).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, mr)).To(Succeed())

			Expect(mr).To(Equal(&resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:            name,
					Namespace:       namespace,
					Labels:          utils.MergeStringMaps(existingLabels, labels),
					Annotations:     utils.MergeStringMaps(existingAnnotations, annotations),
					ResourceVersion: "2",
				},
			}))
		})

		It("should fail updating the managed resource if it doesn't exists", func() {
			mr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}

			Expect(
				NewManagedResource(fakeClient).
					WithNamespacedName(namespace, name).
					WithLabels(labels).
					WithAnnotations(annotations).
					CreateIfNotExists(false).
					Reconcile(ctx),
			).To(BeNotFoundError())

			Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, mr)).To(BeNotFoundError())
		})
	})
})
