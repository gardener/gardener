// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package util_test

import (
	"context"
	"crypto"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	authenticationv1 "k8s.io/api/authentication/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	corev1fake "k8s.io/client-go/kubernetes/typed/core/v1/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/testing"
	"k8s.io/client-go/util/keyutil"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	bootstraptokenutil "k8s.io/cluster-bootstrap/token/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	. "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/kubernetes/certificatesigningrequest"
	"github.com/gardener/gardener/pkg/utils/test"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("Util", func() {
	var log = logr.Discard()

	Describe("#DigestedName", func() {
		It("digest should start with `seed-csr-`", func() {
			privateKeyData, err := keyutil.MakeEllipticPrivateKeyPEM()
			Expect(err).ToNot(HaveOccurred())

			privateKey, err := keyutil.ParsePrivateKeyPEM(privateKeyData)
			Expect(err).ToNot(HaveOccurred())

			signer, ok := privateKey.(crypto.Signer)
			Expect(ok).To(BeTrue())

			organization := "test-org"
			subject := &pkix.Name{
				Organization: []string{organization},
				CommonName:   "test-cn",
			}
			digest, err := certificatesigningrequest.DigestedName(signer.Public(), subject, []certificatesv1.KeyUsage{certificatesv1.UsageDigitalSignature}, "seed-csr-")
			Expect(err).ToNot(HaveOccurred())
			Expect(strings.HasPrefix(digest, "seed-csr-")).To(BeTrue())
		})

		It("should return an error because the public key cannot be marshalled", func() {
			_, err := certificatesigningrequest.DigestedName([]byte("test"), nil, []certificatesv1.KeyUsage{certificatesv1.UsageDigitalSignature}, "seed-csr-")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Util Tests requiring a mock client", func() {
		var (
			ctrl *gomock.Controller
			c    *mockclient.MockClient
			ctx  = context.TODO()

			secretKey client.ObjectKey
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			c = mockclient.NewMockClient(ctrl)

			c.EXPECT().Scheme().Return(kubernetes.GardenScheme).AnyTimes()

			secretKey = client.ObjectKey{
				Namespace: "garden",
				Name:      "secret",
			}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		Describe("#GetKubeconfigFromSecret", func() {
			It("should not return an error because the secret does not exist", func() {
				c.EXPECT().
					Get(ctx, secretKey, gomock.AssignableToTypeOf(&corev1.Secret{})).
					Return(apierrors.NewNotFound(schema.GroupResource{Resource: "Secret"}, secretKey.Name))

				kubeconfig, err := GetKubeconfigFromSecret(ctx, c, secretKey)

				Expect(kubeconfig).To(BeNil())
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not return an error", func() {
				kubeconfigContent := []byte("testing")

				c.EXPECT().
					Get(ctx, secretKey, gomock.AssignableToTypeOf(&corev1.Secret{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, secret *corev1.Secret, _ ...client.GetOption) error {
						secret.Name = secretKey.Name
						secret.Namespace = secretKey.Namespace
						secret.Data = map[string][]byte{
							kubernetes.KubeConfig: kubeconfigContent,
						}
						return nil
					})

				kubeconfig, err := GetKubeconfigFromSecret(ctx, c, secretKey)

				Expect(kubeconfig).To(Equal(kubeconfigContent))
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Describe("#UpdateGardenKubeconfigSecret", func() {
			var (
				certClientConfig *rest.Config
				expectedSecret   *corev1.Secret
			)

			BeforeEach(func() {
				certClientConfig = &rest.Config{Host: "testhost", TLSClientConfig: rest.TLSClientConfig{
					Insecure: false,
					CAFile:   "filepath",
				}}

				expectedSecret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretKey.Name,
						Namespace: secretKey.Namespace,
					},
				}
			})

			It("should create the kubeconfig secret", func() {
				c.EXPECT().
					Get(ctx, secretKey, gomock.AssignableToTypeOf(&corev1.Secret{})).
					Return(apierrors.NewNotFound(schema.GroupResource{Resource: "Secret"}, secretKey.Name))

				expectedKubeconfig, err := CreateGardenletKubeconfigWithClientCertificate(certClientConfig, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				expectedSecret.Data = map[string][]byte{kubernetes.KubeConfig: expectedKubeconfig}

				c.EXPECT().Create(ctx, expectedSecret)

				kubeconfig, err := UpdateGardenKubeconfigSecret(ctx, certClientConfig, nil, nil, c, secretKey)
				Expect(err).ToNot(HaveOccurred())
				Expect(kubeconfig).To(Equal(expectedKubeconfig))
			})

			It("should update the kubeconfig secret", func() {
				expectedSecret.Annotations = map[string]string{"gardener.cloud/operation": "renew"}

				c.EXPECT().
					Get(ctx, secretKey, gomock.AssignableToTypeOf(&corev1.Secret{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
						expectedSecret.DeepCopyInto(obj)
						return nil
					})

				expectedKubeconfig, err := CreateGardenletKubeconfigWithClientCertificate(certClientConfig, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				expectedCopy := expectedSecret.DeepCopy()
				delete(expectedCopy.Annotations, "gardener.cloud/operation")
				expectedCopy.Data = map[string][]byte{kubernetes.KubeConfig: expectedKubeconfig}
				test.EXPECTPatch(ctx, c, expectedCopy, expectedSecret, types.MergePatchType)

				kubeconfig, err := UpdateGardenKubeconfigSecret(ctx, certClientConfig, nil, nil, c, secretKey)
				Expect(err).ToNot(HaveOccurred())
				Expect(kubeconfig).To(Equal(expectedKubeconfig))
			})
		})

		Describe("#UpdateGardenKubeconfigCAIfChanged", func() {
			var (
				secretReference        corev1.SecretReference
				certClientConfig       *rest.Config
				expectedSecret         *corev1.Secret
				gardenClientConnection *config.GardenClientConnection
			)

			BeforeEach(func() {
				secretReference = corev1.SecretReference{
					Name:      "secret",
					Namespace: "garden",
				}

				certClientConfig = &rest.Config{Host: "testhost", TLSClientConfig: rest.TLSClientConfig{
					Insecure: false,
					CAData:   []byte("foo"),
				}}

				expectedSecret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretReference.Name,
						Namespace: secretReference.Namespace,
					},
				}

				gardenClientConnection = &config.GardenClientConnection{
					KubeconfigSecret: &secretReference,
				}
			})

			It("should update the secret if the CA has changed", func() {
				gardenClientConnection.GardenClusterCACert = []byte("bar")

				expectedKubeconfig, err := CreateGardenletKubeconfigWithClientCertificate(certClientConfig, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				updatedCertClientConfig := &rest.Config{Host: "testhost", TLSClientConfig: rest.TLSClientConfig{
					Insecure: false,
					CAData:   gardenClientConnection.GardenClusterCACert,
				}}
				expectedUpdatedKubeconfig, err := CreateGardenletKubeconfigWithClientCertificate(updatedCertClientConfig, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				updatedSecret := expectedSecret.DeepCopy()
				expectedSecret.Data = map[string][]byte{kubernetes.KubeConfig: expectedKubeconfig}
				updatedSecret.Data = map[string][]byte{kubernetes.KubeConfig: expectedUpdatedKubeconfig}

				c.EXPECT().
					Get(ctx, secretKey, gomock.AssignableToTypeOf(&corev1.Secret{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
						expectedSecret.DeepCopyInto(obj)
						return nil
					})
				test.EXPECTPatch(ctx, c, updatedSecret, expectedSecret, types.MergePatchType)

				updatedKubeconfig, err := UpdateGardenKubeconfigCAIfChanged(ctx, log, c, expectedKubeconfig, gardenClientConnection)
				Expect(err).ToNot(HaveOccurred())
				Expect(updatedKubeconfig).ToNot(Equal(expectedKubeconfig))
				Expect(updatedKubeconfig).To(Equal(expectedUpdatedKubeconfig))
			})

			It("should not update the secret if the CA didn't change", func() {
				gardenClientConnection.GardenClusterCACert = certClientConfig.CAData

				expectedKubeconfig, err := CreateGardenletKubeconfigWithClientCertificate(certClientConfig, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				updatedKubeconfig, err := UpdateGardenKubeconfigCAIfChanged(ctx, log, c, expectedKubeconfig, gardenClientConnection)
				Expect(err).ToNot(HaveOccurred())
				Expect(updatedKubeconfig).To(Equal(expectedKubeconfig))
			})

			It("should update the secret if the CA has been removed (via 'none')", func() {
				gardenClientConnection.GardenClusterCACert = []byte("none")

				expectedKubeconfig, err := CreateGardenletKubeconfigWithClientCertificate(certClientConfig, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				updatedCertClientConfig := &rest.Config{Host: "testhost", TLSClientConfig: rest.TLSClientConfig{
					Insecure: false,
					CAData:   []byte{},
				}}
				expectedUpdatedKubeconfig, err := CreateGardenletKubeconfigWithClientCertificate(updatedCertClientConfig, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				updatedSecret := expectedSecret.DeepCopy()
				expectedSecret.Data = map[string][]byte{kubernetes.KubeConfig: expectedKubeconfig}
				updatedSecret.Data = map[string][]byte{kubernetes.KubeConfig: expectedUpdatedKubeconfig}

				c.EXPECT().
					Get(ctx, secretKey, gomock.AssignableToTypeOf(&corev1.Secret{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
						expectedSecret.DeepCopyInto(obj)
						return nil
					})
				test.EXPECTPatch(ctx, c, updatedSecret, expectedSecret, types.MergePatchType)

				updatedKubeconfig, err := UpdateGardenKubeconfigCAIfChanged(ctx, log, c, expectedKubeconfig, gardenClientConnection)
				Expect(err).ToNot(HaveOccurred())
				Expect(updatedKubeconfig).ToNot(Equal(expectedKubeconfig))
				Expect(updatedKubeconfig).To(Equal(expectedUpdatedKubeconfig))
			})

			It("should update the secret if the CA has been removed (via 'null')", func() {
				gardenClientConnection.GardenClusterCACert = []byte("null")

				expectedKubeconfig, err := CreateGardenletKubeconfigWithClientCertificate(certClientConfig, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				updatedCertClientConfig := &rest.Config{Host: "testhost", TLSClientConfig: rest.TLSClientConfig{
					Insecure: false,
					CAData:   []byte{},
				}}
				expectedUpdatedKubeconfig, err := CreateGardenletKubeconfigWithClientCertificate(updatedCertClientConfig, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				updatedSecret := expectedSecret.DeepCopy()
				expectedSecret.Data = map[string][]byte{kubernetes.KubeConfig: expectedKubeconfig}
				updatedSecret.Data = map[string][]byte{kubernetes.KubeConfig: expectedUpdatedKubeconfig}

				c.EXPECT().
					Get(ctx, secretKey, gomock.AssignableToTypeOf(&corev1.Secret{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
						expectedSecret.DeepCopyInto(obj)
						return nil
					})
				test.EXPECTPatch(ctx, c, updatedSecret, expectedSecret, types.MergePatchType)

				updatedKubeconfig, err := UpdateGardenKubeconfigCAIfChanged(ctx, log, c, expectedKubeconfig, gardenClientConnection)
				Expect(err).ToNot(HaveOccurred())
				Expect(updatedKubeconfig).ToNot(Equal(expectedKubeconfig))
				Expect(updatedKubeconfig).To(Equal(expectedUpdatedKubeconfig))
			})
		})

		Describe("#ComputeGardenletKubeconfigWithBootstrapToken", func() {
			var (
				restConfig = &rest.Config{
					Host: "apiserver.dummy",
				}
				seedName                 = "sweet-seed"
				description              = "some"
				validity                 = 24 * time.Hour
				tokenID                  = utils.ComputeSHA256Hex([]byte(seedName))[:6]
				bootstrapTokenSecretName = bootstraptokenutil.BootstrapTokenSecretName(tokenID)
				timestampInTheFuture     = time.Now().UTC().Add(15 * time.Hour).Format(time.RFC3339)
				timestampInThePast       = time.Now().UTC().Add(-15 * time.Hour).Format(time.RFC3339)
			)

			It("should successfully refresh the bootstrap token", func() {
				// There are 3 calls requesting the same secret in the code. This can be improved.
				// However it is not critical as bootstrap token generation does not happen too frequently
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: bootstrapTokenSecretName}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, s *corev1.Secret, _ ...client.GetOption) error {
					s.Data = map[string][]byte{
						bootstraptokenapi.BootstrapTokenExpirationKey: []byte(timestampInThePast),
					}
					return nil
				})
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: bootstrapTokenSecretName}, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(nil).Times(2)

				c.EXPECT().Patch(ctx, gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, s *corev1.Secret, _ client.Patch, _ ...client.PatchOption) error {
						Expect(s.Name).To(Equal(bootstrapTokenSecretName))
						Expect(s.Namespace).To(Equal(metav1.NamespaceSystem))
						Expect(s.Type).To(Equal(bootstraptokenapi.SecretTypeBootstrapToken))
						Expect(s.Data).ToNot(BeNil())
						Expect(s.Data[bootstraptokenapi.BootstrapTokenDescriptionKey]).ToNot(BeNil())
						Expect(s.Data[bootstraptokenapi.BootstrapTokenIDKey]).To(Equal([]byte(tokenID)))
						Expect(s.Data[bootstraptokenapi.BootstrapTokenSecretKey]).ToNot(BeNil())
						Expect(s.Data[bootstraptokenapi.BootstrapTokenExpirationKey]).ToNot(BeNil())
						Expect(s.Data[bootstraptokenapi.BootstrapTokenUsageAuthentication]).To(Equal([]byte("true")))
						Expect(s.Data[bootstraptokenapi.BootstrapTokenUsageSigningKey]).To(Equal([]byte("true")))
						return nil
					})

				kubeconfig, err := ComputeGardenletKubeconfigWithBootstrapToken(ctx, c, restConfig, tokenID, description, validity)
				Expect(err).ToNot(HaveOccurred())
				Expect(kubeconfig).ToNot(BeNil())

				rest, err := kubernetes.RESTConfigFromKubeconfig(kubeconfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(rest.Host).To(Equal(restConfig.Host))
			})

			It("should reuse existing bootstrap token", func() {
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: bootstrapTokenSecretName}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, s *corev1.Secret, _ ...client.GetOption) error {
					s.Data = map[string][]byte{
						bootstraptokenapi.BootstrapTokenExpirationKey: []byte(timestampInTheFuture),
						bootstraptokenapi.BootstrapTokenIDKey:         []byte("dummy"),
						bootstraptokenapi.BootstrapTokenSecretKey:     []byte(bootstrapTokenSecretName),
					}
					return nil
				})

				kubeconfig, err := ComputeGardenletKubeconfigWithBootstrapToken(ctx, c, restConfig, tokenID, description, validity)
				Expect(err).ToNot(HaveOccurred())
				Expect(kubeconfig).ToNot(BeNil())

				rest, err := kubernetes.RESTConfigFromKubeconfig(kubeconfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(rest.Host).To(Equal(restConfig.Host))
			})
		})

		Describe("#ComputeGardenletKubeconfigWithServiceAccountToken", func() {
			It("should succeed", func() {
				var (
					restConfig = &rest.Config{
						Host: "apiserver.dummy",
					}
					serviceAccount = &corev1.ServiceAccount{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "gardenlet",
							Namespace: "garden",
						},
					}
					fakeClient   = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
					coreV1Client = &corev1fake.FakeCoreV1{Fake: &testing.Fake{}}
				)

				coreV1Client.AddReactor("create", "serviceaccounts", func(action testing.Action) (bool, runtime.Object, error) {
					if action.GetSubresource() != "token" {
						return false, nil, errors.New("subresource should be 'token'")
					}

					cAction, ok := action.(testing.CreateAction)
					if !ok {
						return false, nil, fmt.Errorf("could not convert action (type %T) to type testing.CreateAction", cAction)
					}

					return true, &authenticationv1.TokenRequest{
						Status: authenticationv1.TokenRequestStatus{
							Token: "some-token",
						},
					}, nil
				})

				kubeconfig, err := ComputeGardenletKubeconfigWithServiceAccountToken(ctx, fakeClient, coreV1Client, restConfig, serviceAccount.Name, serviceAccount.Namespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(kubeconfig).ToNot(BeNil())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), &corev1.ServiceAccount{})).To(Succeed())

				clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
				Expect(fakeClient.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("gardener.cloud:system:seed-bootstrapper:%s:%s", serviceAccount.Namespace, serviceAccount.Name)}, clusterRoleBinding)).To(Succeed())
				Expect(clusterRoleBinding.RoleRef).To(Equal(rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     "gardener.cloud:system:seed-bootstrapper",
				}))
				Expect(clusterRoleBinding.Subjects).To(ConsistOf(rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      serviceAccount.Name,
					Namespace: serviceAccount.Namespace,
				}))

				rest, err := kubernetes.RESTConfigFromKubeconfig(kubeconfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(rest.Host).To(Equal(restConfig.Host))
			})
		})
	})

	Describe("GetSeedName", func() {
		It("should return the configured name", func() {
			name := "test-name"
			result := GetSeedName(&config.SeedConfig{
				SeedTemplate: gardencore.SeedTemplate{
					ObjectMeta: metav1.ObjectMeta{Name: name},
				},
			})
			Expect(result).To(Equal("test-name"))
		})
	})

	Context("cluster role binding name/service account name/token id/description", func() {
		var (
			kind      string
			namespace = "bar"
			name      = "baz"

			clusterRoleNameWithNamespace = "gardener.cloud:system:seed-bootstrapper:" + namespace + ":" + name

			descriptionWithoutNamespace string
			descriptionWithNamespace    string
		)

		BeforeEach(func() {
			kind = "seedmanagement.gardener.cloud/v1alpha1.ManagedSeed resource"
		})

		JustBeforeEach(func() {
			descriptionWithoutNamespace = fmt.Sprintf("A bootstrap token for the Gardenlet for %s %s.", kind, name)
			descriptionWithNamespace = fmt.Sprintf("A bootstrap token for the Gardenlet for %s %s/%s.", kind, namespace, name)
		})

		Describe("#ClusterRoleBindingName", func() {
			It("should return the correct name", func() {
				Expect(ClusterRoleBindingName(namespace, name)).To(Equal(fmt.Sprintf("gardener.cloud:system:seed-bootstrapper:%s:%s", namespace, name)))
			})
		})

		Describe("#ManagedSeedInfoFromClusterRoleBindingName", func() {
			It("should return the expected namespace/name from a cluster role binding name", func() {
				resultNamespace, resultName := ManagedSeedInfoFromClusterRoleBindingName(clusterRoleNameWithNamespace)
				Expect(resultNamespace).To(Equal(namespace))
				Expect(resultName).To(Equal(name))
			})
		})

		Describe("#ServiceAccountName", func() {
			It("should compute the expected name", func() {
				Expect(ServiceAccountName(name)).To(Equal("gardenlet-bootstrap-" + name))
			})
		})

		Describe("#TokenID", func() {
			It("should compute the expected id (w/o namespace", func() {
				Expect(TokenID(metav1.ObjectMeta{Name: name})).To(Equal("baa5a0"))
			})

			It("should compute the expected id (w/ namespace", func() {
				Expect(TokenID(metav1.ObjectMeta{Name: name, Namespace: namespace})).To(Equal("594384"))
			})
		})

		Describe("#Description", func() {
			It("should compute the expected description (w/o namespace)", func() {
				Expect(Description(kind, "", name)).To(Equal(descriptionWithoutNamespace))
			})

			It("should compute the expected description (w/ namespace)", func() {
				Expect(Description(kind, namespace, name)).To(Equal(descriptionWithNamespace))
			})
		})

		Describe("#MetadataFromDescription", func() {
			When("kind is 'ManagedSeed'", func() {
				BeforeEach(func() {
					kind = "seedmanagement.gardener.cloud/v1alpha1.ManagedSeed resource"
				})

				It("should return the expected namespace/name from a description (w/o namespace)", func() {
					kind, resultNamespace, resultName := MetadataFromDescription(descriptionWithoutNamespace)
					Expect(kind).To(Equal("seedmanagement.gardener.cloud/v1alpha1.ManagedSeed resource"))
					Expect(resultNamespace).To(BeEmpty())
					Expect(resultName).To(Equal(name))
				})

				It("should return the expected namespace/name from a description (w/ namespace)", func() {
					kind, resultNamespace, resultName := MetadataFromDescription(descriptionWithNamespace)
					Expect(kind).To(Equal("seedmanagement.gardener.cloud/v1alpha1.ManagedSeed resource"))
					Expect(resultNamespace).To(Equal(namespace))
					Expect(resultName).To(Equal(name))
				})
			})

			When("kind is 'Gardenlet'", func() {
				BeforeEach(func() {
					kind = "seedmanagement.gardener.cloud/v1alpha1.Gardenlet resource"
				})

				It("should return the expected namespace/name from a description (w/o namespace)", func() {
					kind, resultNamespace, resultName := MetadataFromDescription(descriptionWithoutNamespace)
					Expect(kind).To(Equal("seedmanagement.gardener.cloud/v1alpha1.Gardenlet resource"))
					Expect(resultNamespace).To(BeEmpty())
					Expect(resultName).To(Equal(name))
				})

				It("should return the expected namespace/name from a description (w/ namespace)", func() {
					kind, resultNamespace, resultName := MetadataFromDescription(descriptionWithNamespace)
					Expect(kind).To(Equal("seedmanagement.gardener.cloud/v1alpha1.Gardenlet resource"))
					Expect(resultNamespace).To(Equal(namespace))
					Expect(resultName).To(Equal(name))
				})
			})
		})
	})
})
