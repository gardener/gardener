// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package util_test

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	bootstraptokenutil "k8s.io/cluster-bootstrap/token/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("Util", func() {
	var log = logr.Discard()

	Describe("Util Tests requiring a fake client", func() {
		var (
			c   client.Client
			ctx = context.TODO()

			secretKey client.ObjectKey
		)

		BeforeEach(func() {
			c = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

			secretKey = client.ObjectKey{
				Namespace: "garden",
				Name:      "secret",
			}
		})

		Describe("#GetKubeconfigFromSecret", func() {
			It("should not return an error because the secret does not exist", func() {
				kubeconfig, err := GetKubeconfigFromSecret(ctx, c, secretKey)

				Expect(kubeconfig).To(BeNil())
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not return an error", func() {
				kubeconfigContent := []byte("testing")

				Expect(c.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretKey.Name,
						Namespace: secretKey.Namespace,
					},
					Data: map[string][]byte{
						kubernetes.KubeConfig: kubeconfigContent,
					},
				})).To(Succeed())

				kubeconfig, err := GetKubeconfigFromSecret(ctx, c, secretKey)

				Expect(kubeconfig).To(Equal(kubeconfigContent))
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Describe("#UpdateGardenKubeconfigSecret", func() {
			var (
				certClientConfig *rest.Config
			)

			BeforeEach(func() {
				certClientConfig = &rest.Config{Host: "testhost", TLSClientConfig: rest.TLSClientConfig{
					Insecure: false,
					CAFile:   "filepath",
				}}
			})

			It("should create the kubeconfig secret", func() {
				expectedKubeconfig, err := CreateKubeconfigWithClientCertificate(certClientConfig, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				kubeconfig, err := UpdateGardenKubeconfigSecret(ctx, certClientConfig, nil, nil, c, secretKey)
				Expect(err).ToNot(HaveOccurred())
				Expect(kubeconfig).To(Equal(expectedKubeconfig))

				secret := &corev1.Secret{}
				Expect(c.Get(ctx, secretKey, secret)).To(Succeed())
				Expect(secret.Data).To(HaveKeyWithValue(kubernetes.KubeConfig, expectedKubeconfig))
			})

			It("should update the kubeconfig secret", func() {
				// Create existing secret with renew annotation
				existingSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:        secretKey.Name,
						Namespace:   secretKey.Namespace,
						Annotations: map[string]string{"gardener.cloud/operation": "renew"},
					},
				}
				Expect(c.Create(ctx, existingSecret)).To(Succeed())

				expectedKubeconfig, err := CreateKubeconfigWithClientCertificate(certClientConfig, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				kubeconfig, err := UpdateGardenKubeconfigSecret(ctx, certClientConfig, nil, nil, c, secretKey)
				Expect(err).ToNot(HaveOccurred())
				Expect(kubeconfig).To(Equal(expectedKubeconfig))

				// Verify the secret was patched: renew annotation removed, kubeconfig data set
				secret := &corev1.Secret{}
				Expect(c.Get(ctx, secretKey, secret)).To(Succeed())
				Expect(secret.Annotations).NotTo(HaveKey("gardener.cloud/operation"))
				Expect(secret.Data).To(HaveKeyWithValue(kubernetes.KubeConfig, expectedKubeconfig))
			})
		})

		Describe("#UpdateGardenKubeconfigCAIfChanged", func() {
			var (
				certClientConfig       *rest.Config
				gardenClientConnection *gardenletconfigv1alpha1.GardenClientConnection
			)

			BeforeEach(func() {
				certClientConfig = &rest.Config{Host: "testhost", TLSClientConfig: rest.TLSClientConfig{
					Insecure: false,
					CAData:   []byte("foo"),
				}}

				gardenClientConnection = &gardenletconfigv1alpha1.GardenClientConnection{
					KubeconfigSecret: &corev1.SecretReference{
						Name:      secretKey.Name,
						Namespace: secretKey.Namespace,
					},
				}
			})

			It("should update the secret from garden cluster", func() {
				expectedKubeconfig, err := CreateKubeconfigWithClientCertificate(certClientConfig, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				caConfigMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kube-root-ca.crt",
						Namespace: "gardener-system-public",
					},
					Data: map[string]string{
						"ca.crt": "bar",
					},
				}

				updatedCertClientConfig := &rest.Config{Host: "testhost", TLSClientConfig: rest.TLSClientConfig{
					Insecure: false,
					CAData:   []byte(caConfigMap.Data["ca.crt"]),
				}}
				expectedUpdatedKubeconfig, err := CreateKubeconfigWithClientCertificate(updatedCertClientConfig, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				// Create secret with original kubeconfig
				Expect(c.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretKey.Name,
						Namespace: secretKey.Namespace,
					},
					Data: map[string][]byte{kubernetes.KubeConfig: expectedKubeconfig},
				})).To(Succeed())

				// Create the CA configmap in garden cluster
				Expect(c.Create(ctx, caConfigMap)).To(Succeed())

				updatedKubeconfig, err := UpdateGardenKubeconfigCAIfChanged(ctx, log, c, c, expectedKubeconfig, gardenClientConnection)
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
				// Create bootstrap token secret with expired timestamp
				Expect(c.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      bootstrapTokenSecretName,
						Namespace: metav1.NamespaceSystem,
					},
					Data: map[string][]byte{
						bootstraptokenapi.BootstrapTokenExpirationKey: []byte(timestampInThePast),
					},
				})).To(Succeed())

				kubeconfig, err := ComputeGardenletKubeconfigWithBootstrapToken(ctx, c, restConfig, tokenID, description, validity)
				Expect(err).ToNot(HaveOccurred())
				Expect(kubeconfig).ToNot(BeNil())

				rest, err := kubernetes.RESTConfigFromKubeconfig(kubeconfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(rest.Host).To(Equal(restConfig.Host))

				bootstrapSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      bootstrapTokenSecretName,
						Namespace: metav1.NamespaceSystem,
					},
				}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(bootstrapSecret), bootstrapSecret)).To(Succeed())
				Expect(bootstrapSecret.Type).To(Equal(bootstraptokenapi.SecretTypeBootstrapToken))
				Expect(bootstrapSecret.Data).ToNot(BeNil())
				Expect(bootstrapSecret.Data[bootstraptokenapi.BootstrapTokenDescriptionKey]).ToNot(BeNil())
				Expect(bootstrapSecret.Data[bootstraptokenapi.BootstrapTokenIDKey]).To(Equal([]byte(tokenID)))
				Expect(bootstrapSecret.Data[bootstraptokenapi.BootstrapTokenSecretKey]).ToNot(BeNil())
				Expect(bootstrapSecret.Data[bootstraptokenapi.BootstrapTokenExpirationKey]).ToNot(BeNil())
				Expect(bootstrapSecret.Data[bootstraptokenapi.BootstrapTokenUsageAuthentication]).To(Equal([]byte("true")))
				Expect(bootstrapSecret.Data[bootstraptokenapi.BootstrapTokenUsageSigningKey]).To(Equal([]byte("true")))
			})

			It("should reuse existing bootstrap token", func() {
				// Create bootstrap token secret with future expiration
				Expect(c.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      bootstrapTokenSecretName,
						Namespace: metav1.NamespaceSystem,
					},
					Data: map[string][]byte{
						bootstraptokenapi.BootstrapTokenExpirationKey: []byte(timestampInTheFuture),
						bootstraptokenapi.BootstrapTokenIDKey:         []byte("dummy"),
						bootstraptokenapi.BootstrapTokenSecretKey:     []byte(bootstrapTokenSecretName),
					},
				})).To(Succeed())

				kubeconfig, err := ComputeGardenletKubeconfigWithBootstrapToken(ctx, c, restConfig, tokenID, description, validity)
				Expect(err).ToNot(HaveOccurred())
				Expect(kubeconfig).ToNot(BeNil())

				rest, err := kubernetes.RESTConfigFromKubeconfig(kubeconfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(rest.Host).To(Equal(restConfig.Host))

				bootstrapSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      bootstrapTokenSecretName,
						Namespace: metav1.NamespaceSystem,
					},
				}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(bootstrapSecret), bootstrapSecret)).To(Succeed())
				Expect(bootstrapSecret.Data[bootstraptokenapi.BootstrapTokenIDKey]).To(Equal([]byte("dummy")))
				Expect(bootstrapSecret.Data[bootstraptokenapi.BootstrapTokenSecretKey]).To(Equal([]byte(bootstrapTokenSecretName)))
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
					fakeClient = fakeclient.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
						SubResourceCreate: func(ctx context.Context, c client.Client, _ string, obj client.Object, subResource client.Object, _ ...client.SubResourceCreateOption) error {
							tokenRequest, isTokenRequest := subResource.(*authenticationv1.TokenRequest)
							if !isTokenRequest {
								return apierrors.NewBadRequest(fmt.Sprintf("got invalid type %T, expected TokenRequest", subResource))
							}
							if _, isServiceAccount := obj.(*corev1.ServiceAccount); !isServiceAccount {
								return apierrors.NewNotFound(schema.GroupResource{}, "")
							}

							tokenRequest.Status.Token = "some-token"

							return c.Get(ctx, client.ObjectKeyFromObject(obj), obj)
						},
					}).WithScheme(kubernetesscheme.Scheme).Build()
				)

				kubeconfig, err := ComputeGardenletKubeconfigWithServiceAccountToken(ctx, fakeClient, restConfig, serviceAccount.Name, serviceAccount.Namespace)
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
			result := GetSeedName(&gardenletconfigv1alpha1.SeedConfig{
				SeedTemplate: gardencorev1beta1.SeedTemplate{
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
