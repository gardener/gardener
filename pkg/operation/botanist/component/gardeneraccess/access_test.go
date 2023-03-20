// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardeneraccess_test

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
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/gardeneraccess"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Access", func() {
	var (
		fakeClient client.Client
		sm         secretsmanager.Interface
		access     component.Deployer

		ctx       = context.TODO()
		namespace = "shoot--foo--bar"

		clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  name: gardener.cloud:system:gardener
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: gardener
  namespace: kube-system
- kind: ServiceAccount
  name: gardener-internal
  namespace: kube-system
`

		gardenerSecretName         = "gardener"
		gardenerInternalSecretName = "gardener-internal"
		managedResourceName        = "shoot-core-gardeneraccess"
		managedResourceSecretName  = "managedresource-shoot-core-gardeneraccess"

		serverOutOfCluster = "out-of-cluster"
		serverInCluster    = "in-cluster"

		expectedGardenerSecret         *corev1.Secret
		expectedGardenerInternalSecret *corev1.Secret
		expectedManagedResourceSecret  *corev1.Secret
		expectedManagedResource        *resourcesv1alpha1.ManagedResource
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(fakeClient, namespace)

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: namespace}})).To(Succeed())

		access = New(fakeClient, namespace, sm, Values{
			ServerOutOfCluster: serverOutOfCluster,
			ServerInCluster:    serverInCluster,
		})

		expectedGardenerSecret = &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            gardenerSecretName,
				Namespace:       namespace,
				ResourceVersion: "1",
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":      "gardener",
					"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
				},
				Labels: map[string]string{
					"resources.gardener.cloud/purpose": "token-requestor",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{"kubeconfig": []byte(`apiVersion: v1
clusters:
- cluster:
    server: https://` + serverOutOfCluster + `
  name: ` + namespace + `
contexts:
- context:
    cluster: ` + namespace + `
    user: ` + namespace + `
  name: ` + namespace + `
current-context: ` + namespace + `
kind: Config
preferences: {}
users:
- name: ` + namespace + `
  user: {}
`)},
		}

		expectedGardenerInternalSecret = &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            gardenerInternalSecretName,
				Namespace:       namespace,
				ResourceVersion: "1",
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":      "gardener-internal",
					"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
				},
				Labels: map[string]string{
					"resources.gardener.cloud/purpose": "token-requestor",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{"kubeconfig": []byte(`apiVersion: v1
clusters:
- cluster:
    server: https://` + serverInCluster + `
  name: ` + namespace + `
contexts:
- context:
    cluster: ` + namespace + `
    user: ` + namespace + `
  name: ` + namespace + `
current-context: ` + namespace + `
kind: Config
preferences: {}
users:
- name: ` + namespace + `
  user: {}
`)},
		}

		expectedManagedResourceSecret = &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            managedResourceSecretName,
				Namespace:       namespace,
				ResourceVersion: "1",
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"clusterrolebinding____gardener.cloud_system_gardener.yaml": []byte(clusterRoleBindingYAML),
			},
		}
		expectedManagedResource = &resourcesv1alpha1.ManagedResource{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "resources.gardener.cloud/v1alpha1",
				Kind:       "ManagedResource",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            managedResourceName,
				Namespace:       namespace,
				ResourceVersion: "1",
				Labels:          map[string]string{"origin": "gardener"},
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				SecretRefs: []corev1.LocalObjectReference{
					{Name: managedResourceSecretName},
				},
				InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
				KeepObjects:  pointer.Bool(true),
			},
		}
	})

	AfterEach(func() {
		Expect(fakeClient.Delete(ctx, expectedGardenerSecret)).To(Or(Succeed(), BeNotFoundError()))
		Expect(fakeClient.Delete(ctx, expectedGardenerInternalSecret)).To(Or(Succeed(), BeNotFoundError()))
		Expect(fakeClient.Delete(ctx, expectedManagedResourceSecret)).To(Or(Succeed(), BeNotFoundError()))
		Expect(fakeClient.Delete(ctx, expectedManagedResource)).To(Or(Succeed(), BeNotFoundError()))
	})

	Describe("#Deploy", func() {
		It("should successfully deploy all resources", func() {
			Expect(access.Deploy(ctx)).To(Succeed())

			reconciledGardenerSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: gardenerSecretName, Namespace: namespace}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(reconciledGardenerSecret), reconciledGardenerSecret)).To(Succeed())
			Expect(reconciledGardenerSecret).To(DeepEqual(expectedGardenerSecret))

			reconciledGardenerInternalSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: gardenerInternalSecretName, Namespace: namespace}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(reconciledGardenerInternalSecret), reconciledGardenerInternalSecret)).To(Succeed())
			Expect(reconciledGardenerInternalSecret).To(DeepEqual(expectedGardenerInternalSecret))

			reconciledManagedResourceSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: managedResourceSecretName, Namespace: namespace}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(reconciledManagedResourceSecret), reconciledManagedResourceSecret)).To(Succeed())
			Expect(reconciledManagedResourceSecret).To(DeepEqual(expectedManagedResourceSecret))

			reconciledManagedResource := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceName, Namespace: namespace}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(reconciledManagedResource), reconciledManagedResource)).To(Succeed())
			Expect(reconciledManagedResource).To(DeepEqual(expectedManagedResource))
		})

		It("should remove legacy secret data", func() {
			oldGardenerSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      expectedGardenerSecret.Name,
					Namespace: expectedGardenerSecret.Namespace,
				},
			}
			Expect(fakeClient.Create(ctx, oldGardenerSecret)).To(Succeed())
			expectedGardenerSecret.ResourceVersion = "2"

			oldGardenerInternalSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      expectedGardenerInternalSecret.Name,
					Namespace: expectedGardenerInternalSecret.Namespace,
				},
			}
			Expect(fakeClient.Create(ctx, oldGardenerInternalSecret)).To(Succeed())
			expectedGardenerInternalSecret.ResourceVersion = "2"

			Expect(access.Deploy(ctx)).To(Succeed())

			reconciledGardenerSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: gardenerSecretName, Namespace: namespace}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(reconciledGardenerSecret), reconciledGardenerSecret)).To(Succeed())
			Expect(reconciledGardenerSecret).To(DeepEqual(expectedGardenerSecret))

			reconciledGardenerInternalSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: gardenerInternalSecretName, Namespace: namespace}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(reconciledGardenerInternalSecret), reconciledGardenerInternalSecret)).To(Succeed())
			Expect(reconciledGardenerInternalSecret).To(DeepEqual(expectedGardenerInternalSecret))
		})
	})

	Describe("#Destroy", func() {
		It("should successfully delete all the resources", func() {
			expectedManagedResourceSecret.ResourceVersion = ""
			expectedManagedResource.ResourceVersion = ""

			Expect(fakeClient.Create(ctx, expectedManagedResourceSecret)).To(Succeed())
			Expect(fakeClient.Create(ctx, expectedManagedResource)).To(Succeed())

			Expect(access.Destroy(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedManagedResourceSecret), expectedManagedResourceSecret)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(expectedManagedResource), expectedManagedResource)).To(BeNotFoundError())
		})
	})
})
