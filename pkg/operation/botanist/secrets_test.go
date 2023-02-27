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
	"time"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
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
	mocketcd "github.com/gardener/gardener/pkg/operation/botanist/component/etcd/mock"
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
		})
		botanist.SetShootState(&gardencorev1beta1.ShootState{})
	})

	Describe("#InitializeSecretsManagement", func() {
		Context("when shoot is not in restoration phase", func() {
			It("should generate the certificate authorities and sync the cluster CA to the garden", func() {
				Expect(botanist.InitializeSecretsManagement(ctx)).To(Succeed())

				for _, name := range caSecretNames {
					secret := &corev1.Secret{}
					Expect(seedClient.Get(ctx, kubernetesutils.Key(seedNamespace, name), secret)).To(Succeed())
					verifyCASecret(name, secret, And(HaveKey("ca.crt"), HaveKey("ca.key")))
				}

				gardenSecret := &corev1.Secret{}
				Expect(gardenClient.Get(ctx, kubernetesutils.Key(gardenNamespace, shootName+".ca-cluster"), gardenSecret)).To(Succeed())
				Expect(gardenSecret.Labels).To(HaveKeyWithValue("gardener.cloud/role", "ca-cluster"))
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
				botanist.SetShootState(&gardencorev1beta1.ShootState{
					Spec: gardencorev1beta1.ShootStateSpec{
						Gardener: []gardencorev1beta1.GardenerResourceData{
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

				By("Verify existing CA secrets got restored")
				for _, name := range caSecretNames[:1] {
					secret := &corev1.Secret{}
					Expect(seedClient.Get(ctx, kubernetesutils.Key(seedNamespace, name), secret)).To(Succeed())
					verifyCASecret(name, secret, Equal(map[string][]byte{"data-for": []byte(secret.Name)}))
				}

				By("Verify missing CA secrets got generated")
				for _, name := range caSecretNames[2:] {
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

	Describe("#RenewShootAccessSecrets", func() {
		It("should remove the renew-timestamp annotation from all shoot access secrets", func() {
			var (
				secret1 = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "secret1",
						Namespace:   seedNamespace,
						Annotations: map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": "foo"},
					},
				}
				secret2 = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "secret2",
						Namespace:   seedNamespace,
						Annotations: map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": "foo"},
						Labels:      map[string]string{"resources.gardener.cloud/purpose": "token-requestor"},
					},
				}
				secret3 = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "secret3",
						Namespace:   seedNamespace,
						Annotations: map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": "foo"},
						Labels:      map[string]string{"resources.gardener.cloud/purpose": "token-requestor"},
					},
				}
			)

			Expect(seedClient.Create(ctx, secret1)).To(Succeed())
			Expect(seedClient.Create(ctx, secret2)).To(Succeed())
			Expect(seedClient.Create(ctx, secret3)).To(Succeed())

			Expect(botanist.RenewShootAccessSecrets(ctx)).To(Succeed())

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(secret1), secret1)).To(Succeed())
			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(secret2), secret2)).To(Succeed())
			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(secret3), secret3)).To(Succeed())

			Expect(secret1.Annotations).To(HaveKey("serviceaccount.resources.gardener.cloud/token-renew-timestamp"))
			Expect(secret2.Annotations).NotTo(HaveKey("serviceaccount.resources.gardener.cloud/token-renew-timestamp"))
			Expect(secret3.Annotations).NotTo(HaveKey("serviceaccount.resources.gardener.cloud/token-renew-timestamp"))
		})
	})

	Context("service account signing key secret rotation", func() {
		var (
			namespace1, namespace2 *corev1.Namespace
			sa1, sa2, sa3          *corev1.ServiceAccount
			suffix                 = "-4c6b7a"
		)

		BeforeEach(func() {
			namespace1 = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns1"}}
			namespace2 = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns2"}}

			Expect(shootClient.Create(ctx, namespace1)).To(Succeed())
			Expect(shootClient.Create(ctx, namespace2)).To(Succeed())

			sa1 = &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Name: "sa1", Namespace: namespace1.Name},
				Secrets:    []corev1.ObjectReference{{Name: "sa1secret1"}},
			}
			sa2 = &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Name: "sa2", Namespace: namespace2.Name},
				Secrets:    []corev1.ObjectReference{{Name: "sa2-token" + suffix}, {Name: "sa2secret1"}},
			}
			sa3 = &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Name: "sa3", Namespace: namespace2.Name, Labels: map[string]string{"credentials.gardener.cloud/key-name": "service-account-key-current"}},
				Secrets:    []corev1.ObjectReference{{Name: "sa3secret1"}},
			}

			Expect(shootClient.Create(ctx, sa1)).To(Succeed())
			Expect(shootClient.Create(ctx, sa2)).To(Succeed())
			Expect(shootClient.Create(ctx, sa3)).To(Succeed())
		})

		Describe("#CreateNewServiceAccountSecrets", func() {
			It("should create new service account secrets and make them the first in the list", func() {
				Expect(seedClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "service-account-key-current", Namespace: seedNamespace}})).To(Succeed())

				Expect(botanist.CreateNewServiceAccountSecrets(ctx)).To(Succeed())

				Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(sa1), sa1)).To(Succeed())
				Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(sa2), sa2)).To(Succeed())
				Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(sa3), sa3)).To(Succeed())

				Expect(sa1.Labels).To(HaveKeyWithValue("credentials.gardener.cloud/key-name", "service-account-key-current"))
				Expect(sa2.Labels).NotTo(HaveKeyWithValue("credentials.gardener.cloud/key-name", "service-account-key-current"))
				Expect(sa3.Labels).To(HaveKeyWithValue("credentials.gardener.cloud/key-name", "service-account-key-current"))
				Expect(sa1.Secrets).To(ConsistOf(corev1.ObjectReference{Name: "sa1-token" + suffix}, corev1.ObjectReference{Name: "sa1secret1"}))
				Expect(sa2.Secrets).To(ConsistOf(corev1.ObjectReference{Name: "sa2-token" + suffix}, corev1.ObjectReference{Name: "sa2secret1"}))
				Expect(sa3.Secrets).To(ConsistOf(corev1.ObjectReference{Name: "sa3secret1"}))

				sa1Secret := &corev1.Secret{}
				Expect(shootClient.Get(ctx, kubernetesutils.Key(sa1.Namespace, "sa1-token"+suffix), sa1Secret)).To(Succeed())
				verifyCreatedSATokenSecret(sa1Secret, sa1.Name)
			})
		})

		Describe("#DeleteOldServiceAccountSecrets", func() {
			It("should delete old service account secrets", func() {
				now := time.Now()

				By("Create old ServiceAccount secrets")
				Expect(shootClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
					Name:              "sa1secret1",
					Namespace:         sa1.Namespace,
					CreationTimestamp: metav1.Time{Time: now},
				}})).To(Succeed())
				Expect(shootClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
					Name:              "sa2secret1",
					Namespace:         sa2.Namespace,
					CreationTimestamp: metav1.Time{Time: now},
				}})).To(Succeed())
				Expect(shootClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
					Name:              "sa3secret1",
					Namespace:         sa3.Namespace,
					CreationTimestamp: metav1.Time{Time: now},
				}})).To(Succeed())

				By("Set credentials rotation status in Shoot")
				botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
					Status: gardencorev1beta1.ShootStatus{
						Credentials: &gardencorev1beta1.ShootCredentials{
							Rotation: &gardencorev1beta1.ShootCredentialsRotation{
								ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{
									LastInitiationFinishedTime: &metav1.Time{Time: now.Add(time.Minute)},
								},
							},
						},
					},
				})

				By("Create new ServiceAccount secret")
				Expect(shootClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
					Name:              sa2.Secrets[0].Name,
					Namespace:         sa2.Namespace,
					CreationTimestamp: metav1.Time{Time: botanist.Shoot.GetInfo().Status.Credentials.Rotation.ServiceAccountKey.LastInitiationFinishedTime.Add(time.Minute)},
				}})).To(Succeed())

				By("Run cleanup procedure")
				Expect(botanist.DeleteOldServiceAccountSecrets(ctx)).To(Succeed())

				By("Read ServiceAccounts after running cleanup procedure")
				Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(sa1), sa1)).To(Succeed())
				Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(sa2), sa2)).To(Succeed())
				Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(sa3), sa3)).To(Succeed())

				By("Performing assertions")
				Expect(shootClient.Get(ctx, client.ObjectKey{Name: "sa1secret1", Namespace: sa1.Namespace}, &corev1.Secret{})).To(BeNotFoundError())
				Expect(shootClient.Get(ctx, client.ObjectKey{Name: "sa2secret1", Namespace: sa2.Namespace}, &corev1.Secret{})).To(BeNotFoundError())
				Expect(shootClient.Get(ctx, client.ObjectKey{Name: "sa3secret1", Namespace: sa3.Namespace}, &corev1.Secret{})).To(BeNotFoundError())

				Expect(sa1.Secrets).To(BeEmpty())

				Expect(sa2.Secrets).To(ConsistOf(corev1.ObjectReference{Name: "sa2-token" + suffix}))
				Expect(shootClient.Get(ctx, client.ObjectKey{Name: sa2.Secrets[0].Name, Namespace: sa2.Namespace}, &corev1.Secret{})).To(Succeed())

				Expect(sa3.Labels).NotTo(HaveKey("credentials.gardener.cloud/key-name"))
				Expect(sa3.Secrets).To(BeEmpty())
			})
		})
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

			Expect(shootClient.Create(ctx, namespace1)).To(Succeed())
			Expect(shootClient.Create(ctx, namespace2)).To(Succeed())

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

			Expect(shootClient.Create(ctx, secret1)).To(Succeed())
			Expect(shootClient.Create(ctx, secret2)).To(Succeed())
			Expect(shootClient.Create(ctx, secret3)).To(Succeed())

			kubeAPIServerDeployment = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver", Namespace: seedNamespace}}
			Expect(seedClient.Create(ctx, kubeAPIServerDeployment)).To(Succeed())
		})

		Describe("#RewriteSecretsAddLabel", func() {
			It("should patch all secrets and add the label if not already done", func() {
				Expect(seedClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-etcd-encryption-key-current", Namespace: seedNamespace}})).To(Succeed())

				Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(secret1), secret1)).To(Succeed())
				Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(secret2), secret2)).To(Succeed())
				Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(secret3), secret3)).To(Succeed())

				secret1ResourceVersion := secret1.ResourceVersion
				secret2ResourceVersion := secret2.ResourceVersion
				secret3ResourceVersion := secret3.ResourceVersion

				Expect(botanist.RewriteSecretsAddLabel(ctx)).To(Succeed())

				Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(secret1), secret1)).To(Succeed())
				Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(secret2), secret2)).To(Succeed())
				Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(secret3), secret3)).To(Succeed())

				Expect(secret1.Labels).To(HaveKeyWithValue("credentials.gardener.cloud/key-name", "kube-apiserver-etcd-encryption-key-current"))
				Expect(secret2.Labels).To(HaveKeyWithValue("credentials.gardener.cloud/key-name", "kube-apiserver-etcd-encryption-key-current"))
				Expect(secret3.Labels).To(HaveKeyWithValue("credentials.gardener.cloud/key-name", "kube-apiserver-etcd-encryption-key-current"))

				Expect(secret1.ResourceVersion).NotTo(Equal(secret1ResourceVersion))
				Expect(secret2.ResourceVersion).NotTo(Equal(secret2ResourceVersion))
				Expect(secret3.ResourceVersion).To(Equal(secret3ResourceVersion))
			})
		})

		Describe("#SnapshotETCDAfterRewritingSecrets", func() {
			var (
				ctrl     *gomock.Controller
				etcdMain *mocketcd.MockInterface
			)

			BeforeEach(func() {
				ctrl = gomock.NewController(GinkgoT())
				etcdMain = mocketcd.NewMockInterface(ctrl)

				botanist.Shoot.Components = &shootpkg.Components{
					ControlPlane: &shootpkg.ControlPlane{
						EtcdMain: etcdMain,
					},
				}
			})

			AfterEach(func() {
				ctrl.Finish()
			})

			It("should create a snapshot of ETCD and annotate kube-apiserver accordingly", func() {
				etcdMain.EXPECT().Snapshot(ctx, gomock.Any())

				Expect(botanist.SnapshotETCDAfterRewritingSecrets(ctx)).To(Succeed())

				Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(kubeAPIServerDeployment), kubeAPIServerDeployment)).To(Succeed())
				Expect(kubeAPIServerDeployment.Annotations).To(HaveKeyWithValue("credentials.gardener.cloud/etcd-snapshotted", "true"))
			})
		})

		Describe("#RewriteSecretsRemoveLabel", func() {
			It("should patch all secrets and remove the label if not already done", func() {
				metav1.SetMetaDataAnnotation(&kubeAPIServerDeployment.ObjectMeta, "credentials.gardener.cloud/etcd-snapshotted", "true")
				Expect(seedClient.Update(ctx, kubeAPIServerDeployment)).To(Succeed())

				Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(secret1), secret1)).To(Succeed())
				Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(secret2), secret2)).To(Succeed())
				Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(secret3), secret3)).To(Succeed())

				secret1ResourceVersion := secret1.ResourceVersion
				secret2ResourceVersion := secret2.ResourceVersion
				secret3ResourceVersion := secret3.ResourceVersion

				Expect(botanist.RewriteSecretsRemoveLabel(ctx)).To(Succeed())

				Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(secret1), secret1)).To(Succeed())
				Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(secret2), secret2)).To(Succeed())
				Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(secret3), secret3)).To(Succeed())

				Expect(secret1.Labels).NotTo(HaveKey("credentials.gardener.cloud/key-name"))
				Expect(secret2.Labels).NotTo(HaveKey("credentials.gardener.cloud/key-name"))
				Expect(secret3.Labels).NotTo(HaveKey("credentials.gardener.cloud/key-name"))

				Expect(secret1.ResourceVersion).To(Equal(secret1ResourceVersion))
				Expect(secret2.ResourceVersion).To(Equal(secret2ResourceVersion))
				Expect(secret3.ResourceVersion).NotTo(Equal(secret3ResourceVersion))

				Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(kubeAPIServerDeployment), kubeAPIServerDeployment)).To(Succeed())
				Expect(kubeAPIServerDeployment.Annotations).NotTo(HaveKey("credentials.gardener.cloud/etcd-snapshotted"))
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

func rawData(key, value string) []byte {
	return []byte(`{"` + key + `":"` + utils.EncodeBase64([]byte(value)) + `"}`)
}

func verifyCreatedSATokenSecret(secret *corev1.Secret, serviceAccountName string) {
	ExpectWithOffset(1, secret.Type).To(Equal(corev1.SecretTypeServiceAccountToken))
	ExpectWithOffset(1, secret.Annotations).To(HaveKeyWithValue("kubernetes.io/service-account.name", serviceAccountName))
}
