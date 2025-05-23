// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/utils"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Secrets", func() {
	var (
		ctx = context.TODO()

		shootName             = "bar"
		gardenNamespace       = "garden-foo"
		controlPlaneNamespace = "shoot--foo--bar"

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
		seedClientSet = fakekubernetes.NewClientSetBuilder().WithClient(seedClient).Build()
		shootClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()
		shootClientSet = fakekubernetes.NewClientSetBuilder().WithClient(shootClient).Build()

		fakeSecretsManager = fakesecretsmanager.New(seedClient, controlPlaneNamespace)

		botanist = &Botanist{
			Operation: &operation.Operation{
				Logger:         logr.Discard(),
				GardenClient:   gardenClient,
				SeedClientSet:  seedClientSet,
				ShootClientSet: shootClientSet,
				SecretsManager: fakeSecretsManager,
				Seed:           &seedpkg.Seed{},
				Shoot: &shootpkg.Shoot{
					ControlPlaneNamespace: controlPlaneNamespace,
				},
			},
		}
		botanist.Seed.SetInfo(&gardencorev1beta1.Seed{
			Spec: gardencorev1beta1.SeedSpec{
				Ingress: &gardencorev1beta1.Ingress{
					Domain: "example.com",
				},
			},
		})
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Shoot",
				APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: gardenNamespace,
				UID:       types.UID("daa71cd9-c81a-45ac-a3d3-8bc2f4926a30"),
			},
			Spec: gardencorev1beta1.ShootSpec{
				Provider: gardencorev1beta1.Provider{
					Workers: []gardencorev1beta1.Worker{
						{Name: "foo"},
					},
				},
			},
			Status: gardencorev1beta1.ShootStatus{
				TechnicalID: controlPlaneNamespace,
			},
		})
		botanist.Shoot.SetShootState(&gardencorev1beta1.ShootState{})
	})

	Describe("#DeployCloudProviderSecret", func() {
		It("should create cloud provider secret containing secret data", func() {
			botanist.Shoot.Credentials = &corev1.Secret{
				Data: map[string][]byte{"foo": []byte("bar")},
			}
			Expect(botanist.DeployCloudProviderSecret(ctx)).To(Succeed())

			retrieved := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: controlPlaneNamespace, Name: "cloudprovider"}}
			Expect(botanist.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(retrieved), retrieved)).To(Succeed())
			Expect(retrieved).To(Equal(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       controlPlaneNamespace,
					Name:            "cloudprovider",
					ResourceVersion: "1",
					Labels: map[string]string{
						"gardener.cloud/purpose": "cloudprovider",
					},
				},
				Data: map[string][]byte{"foo": []byte("bar")},
				Type: corev1.SecretTypeOpaque,
			}))
		})

		It("should create cloud provider secret containing WorkloadIdentity data", func() {
			botanist.Shoot.Credentials = &securityv1alpha1.WorkloadIdentity{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wi-name",
					Namespace: "wi-namespace",
				},
				Spec: securityv1alpha1.WorkloadIdentitySpec{
					TargetSystem: securityv1alpha1.TargetSystem{
						Type:           "some-provider",
						ProviderConfig: &runtime.RawExtension{Raw: []byte(`{"raw":"raw"}`)},
					},
				},
			}
			Expect(botanist.DeployCloudProviderSecret(ctx)).To(Succeed())

			retrieved := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: controlPlaneNamespace, Name: "cloudprovider"}}
			Expect(botanist.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(retrieved), retrieved)).To(Succeed())
			Expect(retrieved).To(Equal(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       controlPlaneNamespace,
					Name:            "cloudprovider",
					ResourceVersion: "1",
					Labels: map[string]string{
						"gardener.cloud/purpose":                            "cloudprovider",
						"security.gardener.cloud/purpose":                   "workload-identity-token-requestor",
						"workloadidentity.security.gardener.cloud/provider": "some-provider",
					},
					Annotations: map[string]string{
						"workloadidentity.security.gardener.cloud/namespace":      "wi-namespace",
						"workloadidentity.security.gardener.cloud/name":           "wi-name",
						"workloadidentity.security.gardener.cloud/context-object": `{"kind":"Shoot","apiVersion":"core.gardener.cloud/v1beta1","name":"bar","namespace":"garden-foo","uid":"daa71cd9-c81a-45ac-a3d3-8bc2f4926a30"}`,
					},
				},
				Data: map[string][]byte{"config": []byte(`{"raw":"raw"}`)},
				Type: corev1.SecretTypeOpaque,
			}))
		})

		It("should update the cloud provider secret to contain only WorkloadIdentity data", func() {
			currentSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: controlPlaneNamespace,
					Name:      "cloudprovider",
				},
				Data: map[string][]byte{
					"foo": []byte("bar"),
				},
			}
			Expect(botanist.SeedClientSet.Client().Create(ctx, currentSecret)).To(Succeed())

			botanist.Shoot.Credentials = &securityv1alpha1.WorkloadIdentity{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wi-name",
					Namespace: "wi-namespace",
				},
				Spec: securityv1alpha1.WorkloadIdentitySpec{
					TargetSystem: securityv1alpha1.TargetSystem{
						Type:           "some-provider",
						ProviderConfig: &runtime.RawExtension{Raw: []byte(`{"raw":"raw"}`)},
					},
				},
			}
			Expect(botanist.DeployCloudProviderSecret(ctx)).To(Succeed())

			retrieved := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: controlPlaneNamespace, Name: "cloudprovider"}}
			Expect(botanist.SeedClientSet.Client().Get(ctx, client.ObjectKeyFromObject(retrieved), retrieved)).To(Succeed())
			Expect(retrieved).To(Equal(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       controlPlaneNamespace,
					Name:            "cloudprovider",
					ResourceVersion: "2",
					Labels: map[string]string{
						"gardener.cloud/purpose":                            "cloudprovider",
						"security.gardener.cloud/purpose":                   "workload-identity-token-requestor",
						"workloadidentity.security.gardener.cloud/provider": "some-provider",
					},
					Annotations: map[string]string{
						"workloadidentity.security.gardener.cloud/namespace":      "wi-namespace",
						"workloadidentity.security.gardener.cloud/name":           "wi-name",
						"workloadidentity.security.gardener.cloud/context-object": `{"kind":"Shoot","apiVersion":"core.gardener.cloud/v1beta1","name":"bar","namespace":"garden-foo","uid":"daa71cd9-c81a-45ac-a3d3-8bc2f4926a30"}`,
					},
				},
				Data: map[string][]byte{"config": []byte(`{"raw":"raw"}`)},
				Type: corev1.SecretTypeOpaque,
			}))
		})

		It("should return error when shoot credentials are of unknown type", func() {
			botanist.Shoot.Credentials = &corev1.Pod{}
			Expect(botanist.DeployCloudProviderSecret(ctx)).To(MatchError(Equal("unexpected type *v1.Pod, should be either Secret or WorkloadIdentity")))
		})
	})

	Describe("#InitializeSecretsManagement", func() {
		Context("when shoot is not in restoration phase", func() {
			It("should generate the certificate authorities and sync cluster and client CA to the garden", func() {
				Expect(botanist.InitializeSecretsManagement(ctx)).To(Succeed())

				for _, name := range caSecretNames {
					secret := &corev1.Secret{}
					Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: name}, secret)).To(Succeed())
					verifyCASecret(name, secret, And(HaveKey("ca.crt"), HaveKey("ca.key")))
				}

				gardenConfigMap := &corev1.ConfigMap{}
				Expect(gardenClient.Get(ctx, client.ObjectKey{Namespace: gardenNamespace, Name: shootName + ".ca-cluster"}, gardenConfigMap)).To(Succeed())
				Expect(gardenConfigMap.Labels).To(Equal(
					map[string]string{
						"discovery.gardener.cloud/public":   "shoot-ca",
						"gardener.cloud/role":               "ca-cluster",
						"gardener.cloud/update-restriction": "true",
						"shoot.gardener.cloud/name":         "bar",
						"shoot.gardener.cloud/uid":          "daa71cd9-c81a-45ac-a3d3-8bc2f4926a30",
					},
				))

				if !botanist.Shoot.IsWorkerless {
					gardenConfigMapKubelet := &corev1.ConfigMap{}
					Expect(gardenClient.Get(ctx, client.ObjectKey{Namespace: gardenNamespace, Name: shootName + ".ca-kubelet"}, gardenConfigMapKubelet)).To(Succeed())
					Expect(gardenConfigMapKubelet.Labels).To(Equal(
						map[string]string{
							"gardener.cloud/role":               "ca-kubelet",
							"gardener.cloud/update-restriction": "true",
							"shoot.gardener.cloud/name":         "bar",
							"shoot.gardener.cloud/uid":          "daa71cd9-c81a-45ac-a3d3-8bc2f4926a30",
						},
					))
				}

				gardenSecret := &corev1.Secret{}
				Expect(gardenClient.Get(ctx, client.ObjectKey{Namespace: gardenNamespace, Name: shootName + ".ca-cluster"}, gardenSecret)).To(Succeed())
				Expect(gardenSecret.Labels).To(HaveKeyWithValue("gardener.cloud/role", "ca-cluster"))

				internalSecret := &gardencorev1beta1.InternalSecret{}
				Expect(gardenClient.Get(ctx, client.ObjectKey{Namespace: gardenNamespace, Name: shootName + ".ca-client"}, internalSecret)).To(Succeed())
				Expect(internalSecret.Labels).To(HaveKeyWithValue("gardener.cloud/role", "ca-client"))
				Expect(internalSecret.Data).To(And(HaveKey("ca.crt"), HaveKey("ca.key")))
			})

			It("should generate the generic token kubeconfig", func() {
				Expect(botanist.InitializeSecretsManagement(ctx)).To(Succeed())

				cluster := &extensionsv1alpha1.Cluster{}
				Expect(seedClient.Get(ctx, client.ObjectKey{Name: controlPlaneNamespace}, cluster)).To(Succeed())
				Expect(cluster.Annotations).To(HaveKey("generic-token-kubeconfig.secret.gardener.cloud/name"))

				secret := &corev1.Secret{}
				Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: cluster.Annotations["generic-token-kubeconfig.secret.gardener.cloud/name"]}, secret)).To(Succeed())
			})

			It("should generate the ssh keypair and sync it to the garden", func() {
				Expect(botanist.InitializeSecretsManagement(ctx)).To(Succeed())

				secretList := &corev1.SecretList{}
				Expect(seedClient.List(ctx, secretList, client.InNamespace(controlPlaneNamespace), client.MatchingLabels{
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
				Expect(gardenClient.Get(ctx, client.ObjectKey{Namespace: gardenNamespace, Name: shootName + ".ssh-keypair"}, gardenSecret)).To(Succeed())
				Expect(gardenSecret.Labels).To(HaveKeyWithValue("gardener.cloud/role", "ssh-keypair"))
			})

			It("should not generate the ssh keypair in case of workerless shoot", func() {
				shoot := botanist.Shoot.GetInfo()
				shoot.Spec.Provider.Workers = nil
				botanist.Shoot.SetInfo(shoot)

				Expect(botanist.InitializeSecretsManagement(ctx)).To(Succeed())

				secretList := &corev1.SecretList{}
				Expect(seedClient.List(ctx, secretList, client.InNamespace(controlPlaneNamespace), client.MatchingLabels{
					"name":       "ssh-keypair",
					"managed-by": "secrets-manager",
				})).To(Succeed())
				Expect(secretList.Items).To(BeEmpty())
			})

			It("should also sync the old ssh-keypair secret to the garden", func() {
				Expect(seedClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ssh-keypair-old", Namespace: controlPlaneNamespace}})).To(Succeed())

				Expect(botanist.InitializeSecretsManagement(ctx)).To(Succeed())

				gardenSecret := &corev1.Secret{}
				Expect(gardenClient.Get(ctx, client.ObjectKey{Namespace: gardenNamespace, Name: shootName + ".ssh-keypair.old"}, gardenSecret)).To(Succeed())
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
				Expect(gardenClient.Get(ctx, client.ObjectKey{Namespace: gardenNamespace, Name: shootName + ".ssh-keypair"}, gardenSecret)).To(BeNotFoundError())
				Expect(gardenClient.Get(ctx, client.ObjectKey{Namespace: gardenNamespace, Name: shootName + ".ssh-keypair.old"}, gardenSecret)).To(BeNotFoundError())
			})

			Context("observability credentials", func() {
				It("should generate the password and sync it to the garden", func() {
					botanist.Shoot.WantsAlertmanager = true

					Expect(botanist.InitializeSecretsManagement(ctx)).To(Succeed())

					secretList := &corev1.SecretList{}
					Expect(seedClient.List(ctx, secretList, client.InNamespace(controlPlaneNamespace), client.MatchingLabels{
						"name":       "observability-ingress-users",
						"managed-by": "secrets-manager",
					})).To(Succeed())
					Expect(secretList.Items).To(HaveLen(1))
					Expect(secretList.Items[0].Immutable).To(PointTo(BeTrue()))
					Expect(secretList.Items[0].Labels).To(And(
						HaveKeyWithValue("name", "observability-ingress-users"),
						HaveKeyWithValue("managed-by", "secrets-manager"),
						HaveKeyWithValue("persist", "true"),
						HaveKeyWithValue("rotation-strategy", "inplace"),
						HaveKey("checksum-of-config"),
						HaveKey("last-rotation-initiation-time"),
					))

					gardenSecret := &corev1.Secret{}
					Expect(gardenClient.Get(ctx, client.ObjectKey{Namespace: gardenNamespace, Name: shootName + ".monitoring"}, gardenSecret)).To(Succeed())
					Expect(gardenSecret.Annotations).To(HaveKeyWithValue("url", "https://gu-foo--bar.example.com"))
					Expect(gardenSecret.Annotations).To(HaveKeyWithValue("plutono-url", "https://gu-foo--bar.example.com"))
					Expect(gardenSecret.Annotations).To(HaveKeyWithValue("prometheus-url", "https://p-foo--bar.example.com"))
					Expect(gardenSecret.Annotations).To(HaveKeyWithValue("alertmanager-url", "https://au-foo--bar.example.com"))
					Expect(gardenSecret.Labels).To(HaveKeyWithValue("gardener.cloud/role", "monitoring"))
					Expect(gardenSecret.Data).To(And(HaveKey("username"), HaveKey("password"), HaveKey("auth")))
				})

				It("should not generate the password in case no observability components are needed", func() {
					botanist.Shoot.Purpose = gardencorev1beta1.ShootPurposeTesting

					Expect(botanist.InitializeSecretsManagement(ctx)).To(Succeed())

					secretList := &corev1.SecretList{}
					Expect(seedClient.List(ctx, secretList, client.InNamespace(controlPlaneNamespace), client.MatchingLabels{
						"name":       "observability-ingress-users",
						"managed-by": "secrets-manager",
					})).To(Succeed())
					Expect(secretList.Items).To(BeEmpty())

					gardenSecret := &corev1.Secret{}
					Expect(gardenClient.Get(ctx, client.ObjectKey{Namespace: gardenNamespace, Name: shootName + ".monitoring"}, gardenSecret)).To(BeNotFoundError())
				})
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
					Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: name}, secret)).To(Succeed())
					verifyCASecret(name, secret, Equal(map[string][]byte{"data-for": []byte(secret.Name)}))
				}

				By("Verify missing CA secrets got generated")
				for _, name := range caSecretNames[3:] {
					secret := &corev1.Secret{}
					Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: name}, secret)).To(Succeed())
					verifyCASecret(name, secret, And(HaveKey("ca.crt"), HaveKey("ca.key")))
				}

				By("Verify non-CA secrets got restored")
				secret := &corev1.Secret{}
				Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: "non-ca-secret"}, secret)).To(Succeed())
				Expect(secret.Labels).To(Equal(map[string]string{"managed-by": "secrets-manager", "manager-identity": fakesecretsmanager.ManagerIdentity}))
				Expect(secret.Data).To(Equal(map[string][]byte{"data-for": []byte(secret.Name)}))

				By("Verify external secrets got restored")
				Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: "extension-foo-secret"}, secret)).To(Succeed())
				Expect(secret.Labels).To(Equal(map[string]string{"managed-by": "secrets-manager", "manager-identity": "extension-foo"}))
				Expect(secret.Data).To(Equal(map[string][]byte{"data-for": []byte(secret.Name)}))

				By("Verify unrelated data not to be restored")
				Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: "secret-without-labels"}, &corev1.Secret{})).To(BeNotFoundError())
				Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: controlPlaneNamespace, Name: "some-other-data"}, &corev1.Secret{})).To(BeNotFoundError())
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
