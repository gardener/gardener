// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist_test

import (
	"context"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientset "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = BeforeSuite(func() {
	DeferCleanup(test.WithVar(&secretutils.GenerateKey, secretutils.FakeGenerateKey))
})

var _ = Describe("Secrets", func() {
	var (
		ctx = context.TODO()

		shootName       = "bar"
		gardenNamespace = "garden-foo"
		seedNamespace   = "shoot--foo--bar"

		caSecretNames = []string{
			"ca",
			"ca-etcd",
			"ca-front-proxy",
			"ca-kubelet",
			"ca-metrics-server",
			"ca-vpn",
		}

		fakeGardenClient    client.Client
		fakeGardenInterface kubernetes.Interface

		fakeSeedClient    client.Client
		fakeSeedInterface kubernetes.Interface

		fakeSecretsManager secretsmanager.Interface

		botanist *Botanist
	)

	BeforeEach(func() {
		fakeGardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		fakeGardenInterface = fakeclientset.NewClientSetBuilder().WithClient(fakeGardenClient).Build()

		fakeSeedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeSeedInterface = fakeclientset.NewClientSetBuilder().WithClient(fakeSeedClient).Build()

		fakeSecretsManager = fakesecretsmanager.New(fakeSeedClient, seedNamespace)

		botanist = &Botanist{
			Operation: &operation.Operation{
				K8sGardenClient: fakeGardenInterface,
				K8sSeedClient:   fakeSeedInterface,
				SecretsManager:  fakeSecretsManager,
				Shoot: &shootpkg.Shoot{
					SeedNamespace: seedNamespace,
				},
			},
		}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: gardenNamespace,
			},
		})
		botanist.SetShootState(&gardencorev1alpha1.ShootState{})
	})

	Describe("#InitializeSecretsManagement", func() {
		Context("when shoot is not in restoration phase", func() {
			It("should generate the certificate authorities and sync the cluster CA to the garden", func() {
				Expect(botanist.InitializeSecretsManagement(ctx)).To(Succeed())

				for _, name := range caSecretNames {
					secret := &corev1.Secret{}
					Expect(fakeSeedClient.Get(ctx, kutil.Key(seedNamespace, name), secret)).To(Succeed())
					verifyCASecret(name, secret, And(HaveKey("ca.crt"), HaveKey("ca.key")))
				}

				gardenSecret := &corev1.Secret{}
				Expect(fakeGardenClient.Get(ctx, kutil.Key(gardenNamespace, shootName+".ca-cluster"), gardenSecret)).To(Succeed())
				Expect(gardenSecret.Labels).To(HaveKeyWithValue("gardener.cloud/role", "ca-cluster"))
			})

			It("should generate the generic token kubeconfig", func() {
				Expect(botanist.InitializeSecretsManagement(ctx)).To(Succeed())

				cluster := &extensionsv1alpha1.Cluster{}
				Expect(fakeSeedClient.Get(ctx, kutil.Key(seedNamespace), cluster)).To(Succeed())
				Expect(cluster.Annotations).To(HaveKey("generic-token-kubeconfig.secret.gardener.cloud/name"))

				secret := &corev1.Secret{}
				Expect(fakeSeedClient.Get(ctx, kutil.Key(seedNamespace, cluster.Annotations["generic-token-kubeconfig.secret.gardener.cloud/name"]), secret)).To(Succeed())
			})

			It("should generate the ssh keypair and sync it to the garden", func() {
				Expect(botanist.InitializeSecretsManagement(ctx)).To(Succeed())

				secretList := &corev1.SecretList{}
				Expect(fakeSeedClient.List(ctx, secretList, client.InNamespace(seedNamespace), client.MatchingLabels{
					"name":       "ssh-keypair",
					"managed-by": "secrets-manager",
				})).To(Succeed())
				Expect(secretList.Items).To(HaveLen(1))
				Expect(secretList.Items[0].Immutable).To(PointTo(BeTrue()))
				Expect(secretList.Items[0].Labels).To(And(
					HaveKeyWithValue("name", "ssh-keypair"),
					HaveKeyWithValue("managed-by", "secrets-manager"),
					HaveKeyWithValue("persist", "true"),
					HaveKeyWithValue("rotation-strategy", "keepold"),
					HaveKey("checksum-of-config"),
					HaveKey("last-rotation-initiation-time"),
				))

				gardenSecret := &corev1.Secret{}
				Expect(fakeGardenClient.Get(ctx, kutil.Key(gardenNamespace, shootName+".ssh-keypair"), gardenSecret)).To(Succeed())
				Expect(gardenSecret.Labels).To(HaveKeyWithValue("gardener.cloud/role", "ssh-keypair"))
			})

			It("should also sync the old ssh-keypair secret to the garden", func() {
				Expect(botanist.InitializeSecretsManagement(ctx)).To(Succeed())

				gardenSecret := &corev1.Secret{}
				Expect(fakeGardenClient.Get(ctx, kutil.Key(gardenNamespace, shootName+".ssh-keypair.old"), gardenSecret)).To(Succeed())
				Expect(gardenSecret.Labels).To(HaveKeyWithValue("gardener.cloud/role", "ssh-keypair"))
			})

			It("should delete the legacy ssh-keypair secret", func() {
				legacySecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ssh-keypair", Namespace: seedNamespace}}
				Expect(fakeSeedClient.Create(ctx, legacySecret)).To(Succeed())

				Expect(botanist.InitializeSecretsManagement(ctx)).To(Succeed())

				Expect(fakeSeedClient.Get(ctx, client.ObjectKeyFromObject(legacySecret), &corev1.Secret{})).To(BeNotFoundError())
			})
		})

		Context("when shoot is in restoration phase", func() {
			BeforeEach(func() {
				botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Status: gardencorev1beta1.ShootStatus{
						LastOperation: &gardencorev1beta1.LastOperation{
							Type: gardencorev1beta1.LastOperationTypeRestore,
						},
					},
				})
			})

			It("should restore all secrets from the shootstate", func() {
				botanist.SetShootState(&gardencorev1alpha1.ShootState{
					Spec: gardencorev1alpha1.ShootStateSpec{
						Gardener: []gardencorev1alpha1.GardenerResourceData{
							{
								Name:   "ca",
								Type:   "secret",
								Labels: map[string]string{"managed-by": "secrets-manager", "manager-identity": fakesecretsmanager.ManagerIdentity},
								Data:   runtime.RawExtension{Raw: rawData("data-for", "ca")},
							},
							{
								Name:   "ca-etcd",
								Type:   "secret",
								Labels: map[string]string{"managed-by": "secrets-manager", "manager-identity": fakesecretsmanager.ManagerIdentity},
								Data:   runtime.RawExtension{Raw: rawData("data-for", "ca-etcd")},
							},
							{
								Name:   "non-ca-secret",
								Type:   "secret",
								Labels: map[string]string{"managed-by": "secrets-manager", "manager-identity": fakesecretsmanager.ManagerIdentity},
								Data:   runtime.RawExtension{Raw: rawData("data-for", "non-ca-secret")},
							},
							{
								Name:   "extension-foo-secret",
								Type:   "secret",
								Labels: map[string]string{"managed-by": "secrets-manager", "manager-identity": "extension-foo"},
								Data:   runtime.RawExtension{Raw: rawData("data-for", "extension-foo-secret")},
							},
							{
								Name: "secret-without-labels",
								Type: "secret",
							},
							{
								Name: "some-other-data",
								Type: "not-a-secret",
							},
						},
					},
				})

				Expect(botanist.InitializeSecretsManagement(ctx)).To(Succeed())

				By("verifying existing CA secrets got restored")
				for _, name := range caSecretNames[:1] {
					secret := &corev1.Secret{}
					Expect(fakeSeedClient.Get(ctx, kutil.Key(seedNamespace, name), secret)).To(Succeed())
					verifyCASecret(name, secret, Equal(map[string][]byte{"data-for": []byte(secret.Name)}))
				}

				By("verifying missing CA secrets got generated")
				for _, name := range caSecretNames[2:] {
					secret := &corev1.Secret{}
					Expect(fakeSeedClient.Get(ctx, kutil.Key(seedNamespace, name), secret)).To(Succeed())
					verifyCASecret(name, secret, And(HaveKey("ca.crt"), HaveKey("ca.key")))
				}

				By("verifying non-CA secrets got restored")
				secret := &corev1.Secret{}
				Expect(fakeSeedClient.Get(ctx, kutil.Key(seedNamespace, "non-ca-secret"), secret)).To(Succeed())
				Expect(secret.Labels).To(Equal(map[string]string{"managed-by": "secrets-manager", "manager-identity": fakesecretsmanager.ManagerIdentity}))
				Expect(secret.Data).To(Equal(map[string][]byte{"data-for": []byte(secret.Name)}))

				By("verifying external secrets got restored")
				Expect(fakeSeedClient.Get(ctx, kutil.Key(seedNamespace, "extension-foo-secret"), secret)).To(Succeed())
				Expect(secret.Labels).To(Equal(map[string]string{"managed-by": "secrets-manager", "manager-identity": "extension-foo"}))
				Expect(secret.Data).To(Equal(map[string][]byte{"data-for": []byte(secret.Name)}))

				By("verifying unrelated data not to be restored")
				Expect(fakeSeedClient.Get(ctx, kutil.Key(seedNamespace, "secret-without-labels"), &corev1.Secret{})).To(BeNotFoundError())
				Expect(fakeSeedClient.Get(ctx, kutil.Key(seedNamespace, "some-other-data"), &corev1.Secret{})).To(BeNotFoundError())
			})
		})
	})
})

func verifyCASecret(name string, secret *corev1.Secret, dataMatcher gomegatypes.GomegaMatcher) {
	Expect(secret.Immutable).To(PointTo(BeTrue()))
	Expect(secret.Labels).To(And(
		HaveKeyWithValue("name", name),
		HaveKeyWithValue("managed-by", "secrets-manager"),
		HaveKeyWithValue("manager-identity", fakesecretsmanager.ManagerIdentity),
		HaveKeyWithValue("persist", "true"),
		HaveKeyWithValue("rotation-strategy", "keepold"),
		HaveKey("checksum-of-config"),
		HaveKey("last-rotation-initiation-time"),
	))

	if dataMatcher != nil {
		Expect(secret.Data).To(dataMatcher)
	}
}

func rawData(key, value string) []byte {
	return []byte(`{"` + key + `":"` + utils.EncodeBase64([]byte(value)) + `"}`)
}
