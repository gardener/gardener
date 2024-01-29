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

package app

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
)

func TestApp(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gardenlet App Test Suite")
}

var _ = Describe("Recreate Managed Resource Secrets", func() {
	var (
		ctx        = context.TODO()
		fakeClient client.Client

		secret1         *corev1.Secret
		secret2         *corev1.Secret
		expectedSecret1 *corev1.Secret
		expectedSecret2 *corev1.Secret
		tempSecret3     *corev1.Secret
		expectedSecret3 *corev1.Secret
		secret4         *corev1.Secret
		tempSecret4     *corev1.Secret
		expectedSecret4 *corev1.Secret

		secret5 *corev1.Secret
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		secret1 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret1",
				Namespace: "shoot-ns",
				Labels: map[string]string{
					"resources.gardener.cloud/garbage-collectable-reference": "true",
				},
				Finalizers: []string{"resources.gardener.cloud/gardener-resource-manager"},
			},
			Immutable: ptr.To(true),
			Data: map[string][]byte{
				"test": []byte("foo"),
			},
		}
		secret2 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret2",
				Namespace: "garden",
				Labels: map[string]string{
					"resources.gardener.cloud/garbage-collectable-reference": "true",
				},
				Finalizers: []string{"resources.gardener.cloud/gardener-resource-manager"},
			},
			Immutable: ptr.To(true),
			Data: map[string][]byte{
				"test": []byte("bar"),
			},
		}

		tempSecret3 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret3-temp",
				Namespace: "garden",
				Labels: map[string]string{
					"resources.gardener.cloud/garbage-collectable-reference": "true",
					"resources.gardener.cloud/temp-secret":                   "true",
				},
				Annotations: map[string]string{
					"resources.gardener.cloud/temp-secret-old-name": "secret3",
				},
			},
			Immutable: ptr.To(true),
			Data: map[string][]byte{
				"test": []byte("bar1"),
			},
		}

		secret4 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret4",
				Namespace: "garden",
				Labels: map[string]string{
					"resources.gardener.cloud/garbage-collectable-reference": "true",
				},
				Finalizers: []string{"resources.gardener.cloud/gardener-resource-manager"},
			},
			Immutable: ptr.To(true),
			Data: map[string][]byte{
				"test": []byte("bar1"),
			},
		}

		tempSecret4 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret4-temp",
				Namespace: "garden",
				Labels: map[string]string{
					"resources.gardener.cloud/garbage-collectable-reference": "true",
					"resources.gardener.cloud/temp-secret":                   "true",
				},
				Annotations: map[string]string{
					"resources.gardener.cloud/temp-secret-old-name": "secret4",
				},
			},
			Immutable: ptr.To(true),
			Data: map[string][]byte{
				"test": []byte("bar1"),
			},
		}

		expectedSecret1 = &corev1.Secret{
			TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret1",
				Namespace: "shoot-ns",
				Labels: map[string]string{
					"resources.gardener.cloud/garbage-collectable-reference": "true",
				},
				ResourceVersion: "1",
			},
			Immutable: ptr.To(true),
			Data: map[string][]byte{
				"test": []byte("foo"),
			},
		}

		expectedSecret2 = &corev1.Secret{
			TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret2",
				Namespace: "garden",
				Labels: map[string]string{
					"resources.gardener.cloud/garbage-collectable-reference": "true",
				},
				ResourceVersion: "1",
			},
			Immutable: ptr.To(true),
			Data: map[string][]byte{
				"test": []byte("bar"),
			},
		}

		expectedSecret3 = &corev1.Secret{
			TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret3",
				Namespace: "garden",
				Labels: map[string]string{
					"resources.gardener.cloud/garbage-collectable-reference": "true",
				},
				ResourceVersion: "1",
			},
			Immutable: ptr.To(true),
			Data: map[string][]byte{
				"test": []byte("bar1"),
			},
		}

		expectedSecret4 = &corev1.Secret{
			TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret4",
				Namespace: "garden",
				Labels: map[string]string{
					"resources.gardener.cloud/garbage-collectable-reference": "true",
				},
				ResourceVersion: "1",
			},
			Immutable: ptr.To(true),
			Data: map[string][]byte{
				"test": []byte("bar1"),
			},
		}

		secret5 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret5",
				Namespace: "garden",
				Labels: map[string]string{
					"resources.gardener.cloud/garbage-collectable-reference": "true",
				},
				Finalizers: []string{"resources.gardener.cloud/gardener-resource-manager"},
			},
			Immutable: ptr.To(true),
			Data: map[string][]byte{
				"test": []byte("foo"),
			},
		}
	})

	It("should recreate the managed resource secrets", func() {
		Expect(fakeClient.Create(ctx, secret1)).To(Succeed())
		Expect(fakeClient.Create(ctx, secret2)).To(Succeed())
		Expect(fakeClient.Create(ctx, tempSecret3)).To(Succeed())
		Expect(fakeClient.Create(ctx, secret4)).To(Succeed())
		Expect(fakeClient.Create(ctx, tempSecret4)).To(Succeed())

		Expect(fakeClient.Delete(ctx, secret1)).To(Succeed())
		Expect(fakeClient.Delete(ctx, secret2)).To(Succeed())
		Expect(fakeClient.Delete(ctx, secret4)).To(Succeed())

		s1 := &corev1.Secret{}
		s2 := &corev1.Secret{}
		s3 := &corev1.Secret{}
		s4 := &corev1.Secret{}
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret1), s1)).To(Succeed())
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret2), s2)).To(Succeed())
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret2), s4)).To(Succeed())

		Expect(s1.DeletionTimestamp).ToNot(BeNil())
		Expect(s2.DeletionTimestamp).ToNot(BeNil())
		Expect(s4.DeletionTimestamp).ToNot(BeNil())

		Expect(recreateDeletedManagedResourceSecrets(ctx, fakeClient)).To(Succeed())

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedSecret1), s1)).To(Succeed())
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedSecret2), s2)).To(Succeed())
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedSecret3), s3)).To(Succeed())
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedSecret4), s4)).To(Succeed())

		Expect(s1).To(Equal(expectedSecret1))
		Expect(s2).To(Equal(expectedSecret2))
		Expect(s3).To(Equal(expectedSecret3))
		Expect(s4).To(Equal(expectedSecret4))

		secretList := &corev1.SecretList{}
		Expect(fakeClient.List(ctx, secretList)).To(Succeed())
		Expect(secretList.Items).To(HaveLen(4))
	})

	It("should not recreate the managed resource secret", func() {
		Expect(fakeClient.Create(ctx, secret5)).To(Succeed())

		expected := &corev1.Secret{}
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret5), expected)).To(Succeed())

		Expect(recreateDeletedManagedResourceSecrets(ctx, fakeClient)).To(Succeed())

		got := &corev1.Secret{}
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret5), got)).To(Succeed())
		Expect(expected).To(Equal(got))

		secretList := &corev1.SecretList{}
		Expect(fakeClient.List(ctx, secretList)).To(Succeed())
		Expect(secretList.Items).To(HaveLen(1))
	})
})
