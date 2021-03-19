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

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/keyutil"
	baseconfig "k8s.io/component-base/config"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	bootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
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

				expectedKubeconfig, err := bootstraputil.MarshalKubeconfigWithClientCertificate(certClientConfig, nil, nil)
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
