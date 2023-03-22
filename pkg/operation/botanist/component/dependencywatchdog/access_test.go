// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package dependencywatchdog_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/dependencywatchdog"
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

		externalProbeSecretName = "shoot-access-dependency-watchdog-external-probe"
		internalProbeSecretName = "shoot-access-dependency-watchdog-internal-probe"

		serverOutOfCluster = "out-of-cluster"
		serverInCluster    = "in-cluster"

		expectedExternalProbeSecret *corev1.Secret
		expectedInternalProbeSecret *corev1.Secret
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(fakeClient, namespace)

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: namespace}})).To(Succeed())

		access = NewAccess(fakeClient, namespace, sm, AccessValues{
			ServerOutOfCluster: serverOutOfCluster,
			ServerInCluster:    serverInCluster,
		})

		expectedExternalProbeSecret = &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            externalProbeSecretName,
				Namespace:       namespace,
				ResourceVersion: "1",
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":      "dependency-watchdog-external-probe",
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

		expectedInternalProbeSecret = &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            internalProbeSecretName,
				Namespace:       namespace,
				ResourceVersion: "1",
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":      "dependency-watchdog-internal-probe",
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
	})

	AfterEach(func() {
		Expect(fakeClient.Delete(ctx, expectedExternalProbeSecret)).To(Or(Succeed(), BeNotFoundError()))
		Expect(fakeClient.Delete(ctx, expectedInternalProbeSecret)).To(Or(Succeed(), BeNotFoundError()))
	})

	Describe("#Deploy", func() {
		It("should successfully deploy all resources", func() {
			Expect(access.Deploy(ctx)).To(Succeed())

			reconciledExternalProbeSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: externalProbeSecretName, Namespace: namespace}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(reconciledExternalProbeSecret), reconciledExternalProbeSecret)).To(Succeed())
			Expect(reconciledExternalProbeSecret).To(DeepEqual(expectedExternalProbeSecret))

			reconciledInternalProbeSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: internalProbeSecretName, Namespace: namespace}}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(reconciledInternalProbeSecret), reconciledInternalProbeSecret)).To(Succeed())
			Expect(reconciledInternalProbeSecret).To(DeepEqual(expectedInternalProbeSecret))
		})
	})

	Describe("#Destroy", func() {
		It("should delete the secrets", func() {
			reconciledExternalProbeSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: externalProbeSecretName, Namespace: namespace}}
			Expect(fakeClient.Create(ctx, reconciledExternalProbeSecret)).To(Succeed())
			reconciledInternalProbeSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: internalProbeSecretName, Namespace: namespace}}
			Expect(fakeClient.Create(ctx, reconciledInternalProbeSecret)).To(Succeed())

			Expect(access.Destroy(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(reconciledExternalProbeSecret), &corev1.Secret{})).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(reconciledInternalProbeSecret), &corev1.Secret{})).To(BeNotFoundError())
		})
	})
})
