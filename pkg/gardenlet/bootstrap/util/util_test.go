// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package util_test

import (
	"context"
	"crypto"
	"crypto/x509/pkix"
	"fmt"
	"strings"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/keyutil"
	bootstraptokenapi "k8s.io/cluster-bootstrap/token/api"
	bootstraptokenutil "k8s.io/cluster-bootstrap/token/util"
	baseconfig "k8s.io/component-base/config"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	bootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("Util", func() {

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
			digest, err := bootstraputil.DigestedName(signer.Public(), subject, []certificatesv1.KeyUsage{certificatesv1.UsageDigitalSignature})
			Expect(err).ToNot(HaveOccurred())
			Expect(strings.HasPrefix(digest, "seed-csr-")).To(BeTrue())
		})

		It("should return an error because the public key cannot be marshalled", func() {
			_, err := bootstraputil.DigestedName([]byte("test"), nil, []certificatesv1.KeyUsage{certificatesv1.UsageDigitalSignature})
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Util Tests requiring a mock client", func() {
		var (
			ctrl *gomock.Controller
			c    *mockclient.MockClient
			ctx  = context.TODO()
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			c = mockclient.NewMockClient(ctrl)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		Describe("#GetKubeconfigFromSecret", func() {
			var (
				secretName      = "secret"
				secretNamespace = "garden"
				secretReference = corev1.SecretReference{
					Name:      secretName,
					Namespace: secretNamespace,
				}
			)

			It("should not return an error because the secret does not exist", func() {
				c.EXPECT().
					Get(ctx, kutil.Key(secretReference.Namespace, secretReference.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).
					Return(apierrors.NewNotFound(schema.GroupResource{Resource: "Secret"}, secretReference.Name))

				kubeconfig, err := bootstraputil.GetKubeconfigFromSecret(ctx, c, secretNamespace, secretName)

				Expect(kubeconfig).To(BeNil())
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not return an error", func() {
				kubeconfigContent := []byte("testing")

				c.EXPECT().
					Get(ctx, kutil.Key(secretReference.Namespace, secretReference.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, secret *corev1.Secret) error {
						secret.Name = secretReference.Name
						secret.Namespace = secretReference.Namespace
						secret.Data = map[string][]byte{
							kubernetes.KubeConfig: kubeconfigContent,
						}
						return nil
					})

				kubeconfig, err := bootstraputil.GetKubeconfigFromSecret(ctx, c, secretNamespace, secretName)

				Expect(kubeconfig).To(Equal(kubeconfigContent))
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Describe("#UpdateGardenKubeconfigSecret", func() {
			It("should create the kubeconfig secret", func() {
				secretReference := corev1.SecretReference{
					Name:      "secret",
					Namespace: "garden",
				}

				c.EXPECT().
					Get(ctx, kutil.Key(secretReference.Namespace, secretReference.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).
					Return(apierrors.NewNotFound(schema.GroupResource{Resource: "Secret"}, secretReference.Name))

				certClientConfig := &rest.Config{Host: "testhost", TLSClientConfig: rest.TLSClientConfig{
					Insecure: false,
					CAFile:   "filepath",
				}}

				expectedKubeconfig, err := bootstraputil.CreateGardenletKubeconfigWithClientCertificate(certClientConfig, nil, nil)
				Expect(err).ToNot(HaveOccurred())

				expectedSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretReference.Name,
						Namespace: secretReference.Namespace,
					},
					Data: map[string][]byte{kubernetes.KubeConfig: expectedKubeconfig},
				}
				c.EXPECT().Create(ctx, expectedSecret).
					Return(nil)

				gardenClientConnection := &config.GardenClientConnection{
					KubeconfigSecret: &secretReference,
				}

				kubeconfig, err := bootstraputil.UpdateGardenKubeconfigSecret(ctx, certClientConfig, nil, nil, c, gardenClientConnection)

				Expect(err).ToNot(HaveOccurred())
				Expect(kubeconfig).To(Equal(expectedKubeconfig))
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
				c.EXPECT().Get(ctx, kutil.Key(metav1.NamespaceSystem, bootstrapTokenSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, s *corev1.Secret) error {
					s.Data = map[string][]byte{
						bootstraptokenapi.BootstrapTokenExpirationKey: []byte(timestampInThePast),
					}
					return nil
				})
				c.EXPECT().Get(ctx, kutil.Key(metav1.NamespaceSystem, bootstrapTokenSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(nil).Times(2)

				c.EXPECT().Update(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, s *corev1.Secret) error {
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

				kubeconfig, err := bootstraputil.ComputeGardenletKubeconfigWithBootstrapToken(ctx, c, restConfig, tokenID, description, validity)
				Expect(err).ToNot(HaveOccurred())
				Expect(kubeconfig).ToNot(BeNil())

				rest, err := kubernetes.RESTConfigFromKubeconfig(kubeconfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(rest.Host).To(Equal(restConfig.Host))
			})

			It("should reuse existing bootstrap token", func() {
				c.EXPECT().Get(ctx, kutil.Key(metav1.NamespaceSystem, bootstrapTokenSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, s *corev1.Secret) error {
					s.Data = map[string][]byte{
						bootstraptokenapi.BootstrapTokenExpirationKey: []byte(timestampInTheFuture),
						bootstraptokenapi.BootstrapTokenIDKey:         []byte("dummy"),
						bootstraptokenapi.BootstrapTokenSecretKey:     []byte(bootstrapTokenSecretName),
					}
					return nil
				})

				kubeconfig, err := bootstraputil.ComputeGardenletKubeconfigWithBootstrapToken(ctx, c, restConfig, tokenID, description, validity)
				Expect(err).ToNot(HaveOccurred())
				Expect(kubeconfig).ToNot(BeNil())

				rest, err := kubernetes.RESTConfigFromKubeconfig(kubeconfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(rest.Host).To(Equal(restConfig.Host))
			})
		})

		Describe("#ComputeGardenletKubeconfigWithServiceAccountToken", func() {
			var (
				restConfig = &rest.Config{
					Host: "apiserver.dummy",
				}
				serviceAccountName       = "gardenlet"
				serviceAccountSecretName = "service-account-secret"
			)

			It("should fail because the service account token controller has not yet created a secret for the service account", func() {
				c.EXPECT().Get(ctx, kutil.Key("garden", serviceAccountName), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})).Return(apierrors.NewNotFound(schema.GroupResource{}, serviceAccountSecretName))

				c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{})).DoAndReturn(func(_ context.Context, s *corev1.ServiceAccount) error {
					s.Name = serviceAccountName
					s.Namespace = "garden"
					s.Secrets = []corev1.ObjectReference{}
					return nil
				})

				_, err := bootstraputil.ComputeGardenletKubeconfigWithServiceAccountToken(ctx, c, restConfig, serviceAccountName)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("service account token controller has not yet created a secret for the service account"))
			})

			It("should succeed", func() {
				c.EXPECT().Get(ctx, kutil.Key("garden", serviceAccountName), gomock.AssignableToTypeOf(&corev1.ServiceAccount{})).Return(apierrors.NewNotFound(schema.GroupResource{}, serviceAccountSecretName))

				// create service account
				c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.ServiceAccount{})).DoAndReturn(func(_ context.Context, s *corev1.ServiceAccount) error {
					Expect(s.Name).To(Equal(serviceAccountName))
					Expect(s.Namespace).To(Equal("garden"))
					s.Secrets = []corev1.ObjectReference{
						{
							Name: serviceAccountSecretName,
						},
					}
					return nil
				})

				// mock existing service account secret
				c.EXPECT().Get(ctx, kutil.Key("garden", serviceAccountSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, s *corev1.Secret) error {
					s.Data = map[string][]byte{
						"token": []byte("tokenizer"),
					}
					return nil
				})

				// create cluster role binding
				clusterRoleBinding := &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("gardener.cloud:system:seed-bootstrapper:%s", serviceAccountName),
					},
				}
				c.EXPECT().Get(ctx, kutil.Key(clusterRoleBinding.Name), gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{})).Return(apierrors.NewNotFound(schema.GroupResource{}, clusterRoleBinding.Name))

				c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleBinding{})).DoAndReturn(func(_ context.Context, s *rbacv1.ClusterRoleBinding) error {
					expectedClusterRoleBinding := clusterRoleBinding
					expectedClusterRoleBinding.RoleRef = rbacv1.RoleRef{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "ClusterRole",
						Name:     "gardener.cloud:system:seed-bootstrapper",
					}
					expectedClusterRoleBinding.Subjects = []rbacv1.Subject{
						{
							Kind:      "ServiceAccount",
							Name:      serviceAccountName,
							Namespace: "garden",
						},
					}

					Expect(s).To(Equal(expectedClusterRoleBinding))
					return nil
				})

				kubeconfig, err := bootstraputil.ComputeGardenletKubeconfigWithServiceAccountToken(ctx, c, restConfig, serviceAccountName)
				Expect(err).ToNot(HaveOccurred())
				Expect(kubeconfig).ToNot(BeNil())

				rest, err := kubernetes.RESTConfigFromKubeconfig(kubeconfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(rest.Host).To(Equal(restConfig.Host))
			})
		})
	})

	Describe("BuildBootstrapperName", func() {
		It("should return the correct name", func() {
			name := "foo"
			result := bootstraputil.BuildBootstrapperName(name)
			Expect(result).To(Equal(fmt.Sprintf("%s:%s", bootstraputil.GardenerSeedBootstrapper, name)))
		})
	})

	Describe("GetSeedName", func() {
		It("should return the configured name", func() {
			name := "test-name"
			result := bootstraputil.GetSeedName(&config.SeedConfig{
				SeedTemplate: gardencore.SeedTemplate{
					ObjectMeta: metav1.ObjectMeta{Name: name},
				},
			})
			Expect(result).To(Equal("test-name"))
		})

		It("should return the default name", func() {
			result := bootstraputil.GetSeedName(nil)
			Expect(result).To(Equal("<ambiguous>"))
		})
	})

	Describe("GetTargetClusterName", func() {
		It("should return DedicatedSeedKubeconfig", func() {
			result := bootstraputil.GetTargetClusterName(&config.SeedClientConnection{
				ClientConnectionConfiguration: baseconfig.ClientConnectionConfiguration{
					Kubeconfig: "/var/xxx/",
				},
			})
			Expect(result).To(Equal(bootstraputil.DedicatedSeedKubeconfig))
		})

		It("should return InCluster", func() {
			result := bootstraputil.GetTargetClusterName(nil)
			Expect(result).To(Equal(bootstraputil.InCluster))
		})
	})
})
