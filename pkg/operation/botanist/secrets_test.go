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

package botanist_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesfake "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Secrets", func() {
	var (
		ctx = context.TODO()

		shootName       = "bar"
		gardenNamespace = "garden-foo"
		seedNamespace   = "shoot--foo--bar"

		caSecretNames = []string{
			"ca",
			"ca-client",
			"ca-etcd",
			"ca-front-proxy",
			"ca-kubelet",
			"ca-metrics-server",
			"ca-vpn",
		}

		gardenClient   client.Client
		seedClient     client.Client
		seedClientSet  kubernetes.Interface
		shootClient    client.Client
		shootClientSet kubernetes.Interface

		fakeSecretsManager secretsmanager.Interface

		botanist *Botanist
	)

	BeforeEach(func() {
		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		seedClientSet = kubernetesfake.NewClientSetBuilder().WithClient(seedClient).Build()
		shootClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()
		shootClientSet = kubernetesfake.NewClientSetBuilder().WithClient(shootClient).Build()

		fakeSecretsManager = fakesecretsmanager.New(seedClient, seedNamespace)

		botanist = &Botanist{
			Operation: &operation.Operation{
				Logger:         logr.Discard(),
				GardenClient:   gardenClient,
				SeedClientSet:  seedClientSet,
				ShootClientSet: shootClientSet,
				SecretsManager: fakeSecretsManager,
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
			Spec: gardencorev1beta1.ShootSpec{
				Provider: gardencorev1beta1.Provider{
					Workers: []gardencorev1beta1.Worker{
						{Name: "foo"},
					},
				},
			},
		})
		botanist.Shoot.SetShootState(&gardencorev1beta1.ShootState{})
	})

	Describe("#InitializeSecretsManagement", func() {
		Context("when shoot is not in restoration phase", func() {
			It("should generate the certificate authorities and sync cluster and client CA to the garden", func() {
				Expect(botanist.InitializeSecretsManagement(ctx)).To(Succeed())

				for _, name := range caSecretNames {
					secret := &corev1.Secret{}
					Expect(seedClient.Get(ctx, kubernetesutils.Key(seedNamespace, name), secret)).To(Succeed())
					verifyCASecret(name, secret, And(HaveKey("ca.crt"), HaveKey("ca.key")))
				}

				gardenConfigMap := &corev1.ConfigMap{}
				Expect(gardenClient.Get(ctx, kubernetesutils.Key(gardenNamespace, shootName+".ca-cluster"), gardenConfigMap)).To(Succeed())
				Expect(gardenConfigMap.Labels).To(HaveKeyWithValue("gardener.cloud/role", "ca-cluster"))

				gardenSecret := &corev1.Secret{}
				Expect(gardenClient.Get(ctx, kubernetesutils.Key(gardenNamespace, shootName+".ca-cluster"), gardenSecret)).To(Succeed())
				Expect(gardenSecret.Labels).To(HaveKeyWithValue("gardener.cloud/role", "ca-cluster"))

				internalSecret := &gardencorev1beta1.InternalSecret{}
				Expect(gardenClient.Get(ctx, kubernetesutils.Key(gardenNamespace, shootName+".ca-client"), internalSecret)).To(Succeed())
				Expect(internalSecret.Labels).To(HaveKeyWithValue("gardener.cloud/role", "ca-client"))
				Expect(internalSecret.Data).To(And(HaveKey("ca.crt"), HaveKey("ca.key")))
			})

			It("should generate the generic token kubeconfig", func() {
				Expect(botanist.InitializeSecretsManagement(ctx)).To(Succeed())

				cluster := &extensionsv1alpha1.Cluster{}
				Expect(seedClient.Get(ctx, kubernetesutils.Key(seedNamespace), cluster)).To(Succeed())
				Expect(cluster.Annotations).To(HaveKey("generic-token-kubeconfig.secret.gardener.cloud/name"))

				secret := &corev1.Secret{}
				Expect(seedClient.Get(ctx, kubernetesutils.Key(seedNamespace, cluster.Annotations["generic-token-kubeconfig.secret.gardener.cloud/name"]), secret)).To(Succeed())
			})

			It("should generate the ssh keypair and sync it to the garden", func() {
				Expect(botanist.InitializeSecretsManagement(ctx)).To(Succeed())

				secretList := &corev1.SecretList{}
				Expect(seedClient.List(ctx, secretList, client.InNamespace(seedNamespace), client.MatchingLabels{
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
				Expect(gardenClient.Get(ctx, kubernetesutils.Key(gardenNamespace, shootName+".ssh-keypair"), gardenSecret)).To(Succeed())
				Expect(gardenSecret.Labels).To(HaveKeyWithValue("gardener.cloud/role", "ssh-keypair"))
			})

			It("should not generate the ssh keypair in case of workerless shoot", func() {
				shoot := botanist.Shoot.GetInfo()
				shoot.Spec.Provider.Workers = nil
				botanist.Shoot.SetInfo(shoot)

				Expect(botanist.InitializeSecretsManagement(ctx)).To(Succeed())

				secretList := &corev1.SecretList{}
				Expect(seedClient.List(ctx, secretList, client.InNamespace(seedNamespace), client.MatchingLabels{
					"name":       "ssh-keypair",
					"managed-by": "secrets-manager",
				})).To(Succeed())
				Expect(secretList.Items).To(BeEmpty())
			})

			It("should also sync the old ssh-keypair secret to the garden", func() {
				Expect(seedClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ssh-keypair-old", Namespace: seedNamespace}})).To(Succeed())

				Expect(botanist.InitializeSecretsManagement(ctx)).To(Succeed())

				gardenSecret := &corev1.Secret{}
				Expect(gardenClient.Get(ctx, kubernetesutils.Key(gardenNamespace, shootName+".ssh-keypair.old"), gardenSecret)).To(Succeed())
				Expect(gardenSecret.Labels).To(HaveKeyWithValue("gardener.cloud/role", "ssh-keypair"))
			})

			It("should delete ssh-keypair secrets when ssh access is set to false in workers settings", func() {
				Expect(gardenClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: shootName + ".ssh-keypair", Namespace: gardenNamespace}})).To(Succeed())
				Expect(gardenClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: shootName + ".ssh-keypair.old", Namespace: gardenNamespace}})).To(Succeed())

				shoot := botanist.Shoot.GetInfo()
				shoot.Spec = gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						WorkersSettings: &gardencorev1beta1.WorkersSettings{
							SSHAccess: &gardencorev1beta1.SSHAccess{
								Enabled: false,
							},
						},
					},
				}
				botanist.Shoot.SetInfo(shoot)

				Expect(botanist.InitializeSecretsManagement(ctx)).To(Succeed())

				gardenSecret := &corev1.Secret{}
				Expect(gardenClient.Get(ctx, kubernetesutils.Key(gardenNamespace, shootName+".ssh-keypair"), gardenSecret)).To(BeNotFoundError())
				Expect(gardenClient.Get(ctx, kubernetesutils.Key(gardenNamespace, shootName+".ssh-keypair.old"), gardenSecret)).To(BeNotFoundError())
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
				botanist.Shoot.SetShootState(&gardencorev1beta1.ShootState{
					Spec: gardencorev1beta1.ShootStateSpec{
						Gardener: []gardencorev1beta1.GardenerResourceData{
							{
								Name:   "ca",
								Type:   "secret",
								Labels: map[string]string{"managed-by": "secrets-manager", "manager-identity": fakesecretsmanager.ManagerIdentity},
								Data:   runtime.RawExtension{Raw: rawData("ca")},
							},
							{
								Name:   "ca-client",
								Type:   "secret",
								Labels: map[string]string{"managed-by": "secrets-manager", "manager-identity": fakesecretsmanager.ManagerIdentity},
								Data:   runtime.RawExtension{Raw: rawData("ca-client")},
							},
							{
								Name:   "ca-etcd",
								Type:   "secret",
								Labels: map[string]string{"managed-by": "secrets-manager", "manager-identity": fakesecretsmanager.ManagerIdentity},
								Data:   runtime.RawExtension{Raw: rawData("ca-etcd")},
							},
							{
								Name:   "non-ca-secret",
								Type:   "secret",
								Labels: map[string]string{"managed-by": "secrets-manager", "manager-identity": fakesecretsmanager.ManagerIdentity},
								Data:   runtime.RawExtension{Raw: rawData("non-ca-secret")},
							},
							{
								Name:   "extension-foo-secret",
								Type:   "secret",
								Labels: map[string]string{"managed-by": "secrets-manager", "manager-identity": "extension-foo"},
								Data:   runtime.RawExtension{Raw: rawData("extension-foo-secret")},
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

				By("Verify existing CA secrets got restored")
				for _, name := range caSecretNames[:2] {
					secret := &corev1.Secret{}
					Expect(seedClient.Get(ctx, kubernetesutils.Key(seedNamespace, name), secret)).To(Succeed())
					verifyCASecret(name, secret, Equal(map[string][]byte{"data-for": []byte(secret.Name)}))
				}

				By("Verify missing CA secrets got generated")
				for _, name := range caSecretNames[3:] {
					secret := &corev1.Secret{}
					Expect(seedClient.Get(ctx, kubernetesutils.Key(seedNamespace, name), secret)).To(Succeed())
					verifyCASecret(name, secret, And(HaveKey("ca.crt"), HaveKey("ca.key")))
				}

				By("Verify non-CA secrets got restored")
				secret := &corev1.Secret{}
				Expect(seedClient.Get(ctx, kubernetesutils.Key(seedNamespace, "non-ca-secret"), secret)).To(Succeed())
				Expect(secret.Labels).To(Equal(map[string]string{"managed-by": "secrets-manager", "manager-identity": fakesecretsmanager.ManagerIdentity}))
				Expect(secret.Data).To(Equal(map[string][]byte{"data-for": []byte(secret.Name)}))

				By("Verify external secrets got restored")
				Expect(seedClient.Get(ctx, kubernetesutils.Key(seedNamespace, "extension-foo-secret"), secret)).To(Succeed())
				Expect(secret.Labels).To(Equal(map[string]string{"managed-by": "secrets-manager", "manager-identity": "extension-foo"}))
				Expect(secret.Data).To(Equal(map[string][]byte{"data-for": []byte(secret.Name)}))

				By("Verify unrelated data not to be restored")
				Expect(seedClient.Get(ctx, kubernetesutils.Key(seedNamespace, "secret-without-labels"), &corev1.Secret{})).To(BeNotFoundError())
				Expect(seedClient.Get(ctx, kubernetesutils.Key(seedNamespace, "some-other-data"), &corev1.Secret{})).To(BeNotFoundError())
			})
		})
	})
})

func verifyCASecret(name string, secret *corev1.Secret, dataMatcher gomegatypes.GomegaMatcher) {
	ExpectWithOffset(1, secret.Immutable).To(PointTo(BeTrue()))
	ExpectWithOffset(1, secret.Labels).To(And(
		HaveKeyWithValue("name", name),
		HaveKeyWithValue("managed-by", "secrets-manager"),
		HaveKeyWithValue("manager-identity", fakesecretsmanager.ManagerIdentity),
		HaveKeyWithValue("persist", "true"),
		HaveKeyWithValue("rotation-strategy", "keepold"),
		HaveKey("checksum-of-config"),
		HaveKey("last-rotation-initiation-time"),
	))

	if dataMatcher != nil {
		ExpectWithOffset(1, secret.Data).To(dataMatcher)
	}
}

func rawData(value string) []byte {
	return []byte(`{"data-for":"` + utils.EncodeBase64([]byte(value)) + `"}`)
}
