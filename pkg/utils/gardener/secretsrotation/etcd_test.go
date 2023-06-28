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

package secretsrotation_test

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	mocketcd "github.com/gardener/gardener/pkg/component/etcd/mock"
	. "github.com/gardener/gardener/pkg/utils/gardener/secretsrotation"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
)

var _ = Describe("ETCD", func() {
	var (
		ctx    = context.TODO()
		logger logr.Logger

		kubeAPIServerNamespace      = "shoot--foo--bar"
		namePrefix                  = "baz-"
		kubeAPIServerDeploymentName = namePrefix + "kube-apiserver"

		runtimeClient      client.Client
		targetClient       client.Client
		fakeSecretsManager secretsmanager.Interface
	)

	BeforeEach(func() {
		logger = logr.Discard()

		runtimeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		targetClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()
		fakeSecretsManager = fakesecretsmanager.New(runtimeClient, kubeAPIServerNamespace)
	})

	Context("etcd encryption key secret rotation", func() {
		var (
			namespace1, namespace2    *corev1.Namespace
			secret1, secret2, secret3 *corev1.Secret
			kubeAPIServerDeployment   *appsv1.Deployment
		)

		BeforeEach(func() {
			namespace1 = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns1"}}
			namespace2 = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns2"}}

			Expect(targetClient.Create(ctx, namespace1)).To(Succeed())
			Expect(targetClient.Create(ctx, namespace2)).To(Succeed())

			secret1 = &corev1.Secret{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
				ObjectMeta: metav1.ObjectMeta{Name: "secret1", Namespace: namespace1.Name},
			}
			secret2 = &corev1.Secret{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
				ObjectMeta: metav1.ObjectMeta{Name: "secret2", Namespace: namespace2.Name},
			}
			secret3 = &corev1.Secret{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
				ObjectMeta: metav1.ObjectMeta{Name: "secret3", Namespace: namespace2.Name, Labels: map[string]string{"credentials.gardener.cloud/key-name": "kube-apiserver-etcd-encryption-key-current"}},
			}

			Expect(targetClient.Create(ctx, secret1)).To(Succeed())
			Expect(targetClient.Create(ctx, secret2)).To(Succeed())
			Expect(targetClient.Create(ctx, secret3)).To(Succeed())

			kubeAPIServerDeployment = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: kubeAPIServerDeploymentName, Namespace: kubeAPIServerNamespace}}
			Expect(runtimeClient.Create(ctx, kubeAPIServerDeployment)).To(Succeed())
		})

		Describe("#RewriteEncryptedDataAddLabel", func() {
			It("should patch all secrets and add the label if not already done", func() {
				Expect(runtimeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-etcd-encryption-key-current", Namespace: kubeAPIServerNamespace}})).To(Succeed())

				Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(secret1), secret1)).To(Succeed())
				Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(secret2), secret2)).To(Succeed())
				Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(secret3), secret3)).To(Succeed())

				secret1ResourceVersion := secret1.ResourceVersion
				secret2ResourceVersion := secret2.ResourceVersion
				secret3ResourceVersion := secret3.ResourceVersion

				Expect(RewriteEncryptedDataAddLabel(ctx, logger, targetClient, fakeSecretsManager, corev1.SchemeGroupVersion.WithKind("SecretList"))).To(Succeed())

				Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(secret1), secret1)).To(Succeed())
				Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(secret2), secret2)).To(Succeed())
				Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(secret3), secret3)).To(Succeed())

				Expect(secret1.Labels).To(HaveKeyWithValue("credentials.gardener.cloud/key-name", "kube-apiserver-etcd-encryption-key-current"))
				Expect(secret2.Labels).To(HaveKeyWithValue("credentials.gardener.cloud/key-name", "kube-apiserver-etcd-encryption-key-current"))
				Expect(secret3.Labels).To(HaveKeyWithValue("credentials.gardener.cloud/key-name", "kube-apiserver-etcd-encryption-key-current"))

				Expect(secret1.ResourceVersion).NotTo(Equal(secret1ResourceVersion))
				Expect(secret2.ResourceVersion).NotTo(Equal(secret2ResourceVersion))
				Expect(secret3.ResourceVersion).To(Equal(secret3ResourceVersion))
			})
		})

		Describe("#SnapshotETCDAfterRewritingEncryptedData", func() {
			var (
				ctrl     *gomock.Controller
				etcdMain *mocketcd.MockInterface
			)

			BeforeEach(func() {
				ctrl = gomock.NewController(GinkgoT())
				etcdMain = mocketcd.NewMockInterface(ctrl)
			})

			AfterEach(func() {
				ctrl.Finish()
			})

			It("should create a snapshot of ETCD and annotate kube-apiserver accordingly", func() {
				etcdMain.EXPECT().Snapshot(ctx, nil)

				Expect(SnapshotETCDAfterRewritingEncryptedData(ctx, runtimeClient, func(ctx context.Context) error { return etcdMain.Snapshot(ctx, nil) }, kubeAPIServerNamespace, kubeAPIServerDeploymentName)).To(Succeed())

				Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(kubeAPIServerDeployment), kubeAPIServerDeployment)).To(Succeed())
				Expect(kubeAPIServerDeployment.Annotations).To(HaveKeyWithValue("credentials.gardener.cloud/etcd-snapshotted", "true"))
			})
		})

		Describe("#RewriteEncryptedDataRemoveLabel", func() {
			It("should patch all secrets and remove the label if not already done", func() {
				metav1.SetMetaDataAnnotation(&kubeAPIServerDeployment.ObjectMeta, "credentials.gardener.cloud/etcd-snapshotted", "true")
				Expect(runtimeClient.Update(ctx, kubeAPIServerDeployment)).To(Succeed())

				Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(secret1), secret1)).To(Succeed())
				Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(secret2), secret2)).To(Succeed())
				Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(secret3), secret3)).To(Succeed())

				secret1ResourceVersion := secret1.ResourceVersion
				secret2ResourceVersion := secret2.ResourceVersion
				secret3ResourceVersion := secret3.ResourceVersion

				Expect(RewriteEncryptedDataRemoveLabel(ctx, logger, runtimeClient, targetClient, kubeAPIServerNamespace, kubeAPIServerDeploymentName, corev1.SchemeGroupVersion.WithKind("SecretList"))).To(Succeed())

				Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(secret1), secret1)).To(Succeed())
				Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(secret2), secret2)).To(Succeed())
				Expect(targetClient.Get(ctx, client.ObjectKeyFromObject(secret3), secret3)).To(Succeed())

				Expect(secret1.Labels).NotTo(HaveKey("credentials.gardener.cloud/key-name"))
				Expect(secret2.Labels).NotTo(HaveKey("credentials.gardener.cloud/key-name"))
				Expect(secret3.Labels).NotTo(HaveKey("credentials.gardener.cloud/key-name"))

				Expect(secret1.ResourceVersion).To(Equal(secret1ResourceVersion))
				Expect(secret2.ResourceVersion).To(Equal(secret2ResourceVersion))
				Expect(secret3.ResourceVersion).NotTo(Equal(secret3ResourceVersion))

				Expect(runtimeClient.Get(ctx, client.ObjectKeyFromObject(kubeAPIServerDeployment), kubeAPIServerDeployment)).To(Succeed())
				Expect(kubeAPIServerDeployment.Annotations).NotTo(HaveKey("credentials.gardener.cloud/etcd-snapshotted"))
			})
		})
	})
})
