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

package apiserver_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/apiserver"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Admission", func() {
	var (
		ctx       = context.TODO()
		namespace = "some-namespace"

		fakeClient client.Client
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
	})

	Describe("#ReconcileSecretAdmissionKubeconfigs", func() {
		It("should successfully deploy the secret resource w/o admission plugin kubeconfigs", func() {
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "apiserver-admission-kubeconfigs", Namespace: namespace}}
			Expect(kubernetesutils.MakeUnique(secret)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(BeNotFoundError())

			Expect(ReconcileSecretAdmissionKubeconfigs(ctx, fakeClient, secret, Values{})).To(Succeed())

			actualSecret := &corev1.Secret{ObjectMeta: secret.ObjectMeta}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(actualSecret), actualSecret)).To(Succeed())
			Expect(actualSecret).To(DeepEqual(&corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            secret.Name,
					Namespace:       secret.Namespace,
					Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
					ResourceVersion: "1",
				},
				Immutable: pointer.Bool(true),
				Data:      map[string][]byte{},
			}))
		})

		It("should successfully deploy the configmap resource w/ admission plugins", func() {
			admissionPlugins := []AdmissionPluginConfig{
				{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Foo"}},
				{AdmissionPlugin: gardencorev1beta1.AdmissionPlugin{Name: "Baz"}, Kubeconfig: []byte("foo")},
			}

			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "apiserver-admission-kubeconfigs", Namespace: namespace}}
			Expect(kubernetesutils.MakeUnique(secret)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(BeNotFoundError())

			Expect(ReconcileSecretAdmissionKubeconfigs(ctx, fakeClient, secret, Values{EnabledAdmissionPlugins: admissionPlugins})).To(Succeed())

			actualSecret := &corev1.Secret{ObjectMeta: secret.ObjectMeta}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(actualSecret), actualSecret)).To(Succeed())
			Expect(actualSecret).To(DeepEqual(&corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            secret.Name,
					Namespace:       secret.Namespace,
					Labels:          map[string]string{"resources.gardener.cloud/garbage-collectable-reference": "true"},
					ResourceVersion: "1",
				},
				Immutable: pointer.Bool(true),
				Data: map[string][]byte{
					"baz-kubeconfig.yaml": []byte("foo"),
				},
			}))
		})
	})
})
