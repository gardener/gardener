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

package certificate

import (
	"context"
	"crypto/x509/pkix"
	"fmt"
	"net"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mock "github.com/gardener/gardener/pkg/mock/gardener/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Certificates", func() {
	var (
		log logrus.FieldLogger = logger.NewNopLogger()
		ctx                    = context.TODO()

		ctrl                *gomock.Controller
		mockGardenInterface *mock.MockInterface
		mockSeedInterface   *mock.MockInterface

		mockGardenClient *mockclient.MockClient
		mockSeedClient   *mockclient.MockClient

		seedName = "test"
		seed     = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{Name: seedName},
		}

		gardenClientConnection = &config.GardenClientConnection{
			KubeconfigSecret: &corev1.SecretReference{
				Name:      "gardenlet-kubeconfig",
				Namespace: "garden",
			},
		}

		approvedCSR = certificatesv1beta1.CertificateSigningRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name: "watched-csr",
			},
			Status: certificatesv1beta1.CertificateSigningRequestStatus{
				Conditions: []certificatesv1beta1.CertificateSigningRequestCondition{
					{
						Type: certificatesv1beta1.CertificateApproved,
					},
				},
				Certificate: []byte("my-cert"),
			},
		}

		deniedCSR = certificatesv1beta1.CertificateSigningRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name: "watched-csr",
			},
			Status: certificatesv1beta1.CertificateSigningRequestStatus{
				Conditions: []certificatesv1beta1.CertificateSigningRequestCondition{
					{
						Type: certificatesv1beta1.CertificateDenied,
					},
				},
			},
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockSeedInterface = mock.NewMockInterface(ctrl)
		mockSeedClient = mockclient.NewMockClient(ctrl)
		// mockSeedInterface.EXPECT().Client().Return(mockSeedClient).AnyTimes()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#rotateCertificate", func() {
		var certificateSubject = pkix.Name{
			CommonName: x509CommonName,
		}

		BeforeEach(func() {
			mockGardenInterface = mock.NewMockInterface(ctrl)
			mockGardenClient = mockclient.NewMockClient(ctrl)
			mockGardenInterface.EXPECT().Client().Return(mockGardenClient).AnyTimes()

		})

		It("should not return an error", func() {
			// simple Kubernetes client that returns an approved CSR when requested (with watch support)
			// no mock Kubernetes client available that could be easily used
			kubeClient := fake.NewSimpleClientset(&approvedCSR)
			mockGardenInterface.EXPECT().Kubernetes().Return(kubeClient)

			// mock gardenClient.RESTConfig()
			certClientConfig := &rest.Config{Host: "testhost", TLSClientConfig: rest.TLSClientConfig{
				Insecure: false,
				CAFile:   "filepath",
			}}
			mockGardenInterface.EXPECT().RESTConfig().Return(certClientConfig)

			// mock update of secret in seed with the rotated kubeconfig
			mockSeedClient.EXPECT().Get(ctx, kutil.Key(gardenClientConnection.KubeconfigSecret.Namespace, gardenClientConnection.KubeconfigSecret.Name), gomock.AssignableToTypeOf(&corev1.Secret{}))
			mockSeedClient.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, obj runtime.Object, _ ...client.UpdateOption) error {
				secret, ok := obj.(*corev1.Secret)
				Expect(ok).To(BeTrue())
				Expect(secret.Name).To(Equal(gardenClientConnection.KubeconfigSecret.Name))
				Expect(secret.Namespace).To(Equal(gardenClientConnection.KubeconfigSecret.Namespace))
				Expect(secret.Data).ToNot(BeEmpty())
				return nil
			})

			fakeClientMap := fakeclientmap.NewClientMap().
				AddClient(keys.ForGarden(), mockGardenInterface)

			err := rotateCertificate(context.TODO(), log, fakeClientMap, mockSeedClient, gardenClientConnection, &certificateSubject, []string{}, []net.IP{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error - CSR is denied", func() {
			kubeClient := fake.NewSimpleClientset(&deniedCSR)
			mockGardenInterface.EXPECT().Kubernetes().Return(kubeClient)
			fakeClientMap := fakeclientmap.NewClientMap().
				AddClient(keys.ForGarden(), mockGardenInterface).
				AddClient(keys.ForSeed(seed), mockSeedInterface)

			err := rotateCertificate(context.TODO(), log, fakeClientMap, mockSeedClient, gardenClientConnection, &certificateSubject, []string{}, []net.IP{})
			Expect(err).To(HaveOccurred())
		})

		It("should return an error - the CN of the x509 cert to be rotated is not set", func() {
			kubeClient := fake.NewSimpleClientset(&deniedCSR)
			mockGardenInterface.EXPECT().Kubernetes().Return(kubeClient)
			fakeClientMap := fakeclientmap.NewClientMap().
				AddClient(keys.ForGarden(), mockGardenInterface).
				AddClient(keys.ForSeed(seed), mockSeedInterface)

			err := rotateCertificate(context.TODO(), log, fakeClientMap, mockSeedClient, gardenClientConnection, nil, x509DnsNames, x509IpAddresses)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Tests that require a generated kubeconfig with a client certificate", func() {
		var (
			gardenKubeconfigWithValidClientCert   string
			gardenKubeconfigWithInValidClientCert string
			baseKubeconfig                        = `
apiVersion: v1
clusters:
- cluster:
    server: https://localhost:2443
  name: gardenlet
contexts:
- context:
    cluster: gardenlet
    user: gardenlet
  name: gardenlet
current-context: gardenlet
kind: Config
preferences: {}
users:
- name: gardenlet
  user:
    client-certificate-data: %s
    client-key-data: %s
`
		)

		Describe("#getCurrentCertificate", func() {
			BeforeEach(func() {
				// generate kubeconfigs
				validity := 20 * time.Second
				cert := generateCertificate(validity)
				gardenKubeconfigWithValidClientCert = fmt.Sprintf(baseKubeconfig, utils.EncodeBase64(cert.CertificatePEM), utils.EncodeBase64(cert.PrivateKeyPEM))
				gardenKubeconfigWithInValidClientCert = fmt.Sprintf(baseKubeconfig, "bm90LXZhbGlk", utils.EncodeBase64(cert.PrivateKeyPEM))
			})

			It("should not return an error", func() {
				// mock existing secret with garden kubeconfig
				mockSeedClient.EXPECT().Get(ctx, kutil.Key(gardenClientConnection.KubeconfigSecret.Namespace, gardenClientConnection.KubeconfigSecret.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, secret *corev1.Secret) error {
					secret.ObjectMeta = metav1.ObjectMeta{
						Name:      gardenClientConnection.KubeconfigSecret.Name,
						Namespace: gardenClientConnection.KubeconfigSecret.Namespace,
					}
					secret.Data = map[string][]byte{kubernetes.KubeConfig: []byte(gardenKubeconfigWithValidClientCert)}
					return nil
				})

				cert, err := getCurrentCertificate(context.TODO(), log, mockSeedClient, gardenClientConnection)
				Expect(err).ToNot(HaveOccurred())
				Expect(cert).ToNot(BeNil())
			})

			It("should return an error - kubeconfig secret does not exist", func() {
				secretGroupResource := schema.GroupResource{Resource: "Secrets"}
				secretNotFoundErr := apierrors.NewNotFound(secretGroupResource, gardenClientConnection.KubeconfigSecret.Name)
				mockSeedClient.EXPECT().Get(ctx, kutil.Key(gardenClientConnection.KubeconfigSecret.Namespace, gardenClientConnection.KubeconfigSecret.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(secretNotFoundErr)

				_, err := getCurrentCertificate(context.TODO(), log, mockSeedClient, gardenClientConnection)
				Expect(err).To(HaveOccurred())
			})

			It("should return an error - secret does not contain a kubeconfig", func() {
				// mock existing secret with missing garden kubeconfig
				mockSeedClient.EXPECT().Get(ctx, kutil.Key(gardenClientConnection.KubeconfigSecret.Namespace, gardenClientConnection.KubeconfigSecret.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, secret *corev1.Secret) error {
					secret.ObjectMeta = metav1.ObjectMeta{
						Name:      gardenClientConnection.KubeconfigSecret.Name,
						Namespace: gardenClientConnection.KubeconfigSecret.Namespace,
					}
					secret.Data = nil
					return nil
				})

				_, err := getCurrentCertificate(context.TODO(), log, mockSeedClient, gardenClientConnection)
				Expect(err).To(HaveOccurred())
			})

			It("should return an error - kubeconfig client cert is invalid", func() {
				// mock existing secret with garden kubeconfig that has an invalid client cert
				mockSeedClient.EXPECT().Get(ctx, kutil.Key(gardenClientConnection.KubeconfigSecret.Namespace, gardenClientConnection.KubeconfigSecret.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, secret *corev1.Secret) error {
					secret.ObjectMeta = metav1.ObjectMeta{
						Name:      gardenClientConnection.KubeconfigSecret.Name,
						Namespace: gardenClientConnection.KubeconfigSecret.Namespace,
					}
					secret.Data = map[string][]byte{kubernetes.KubeConfig: []byte(gardenKubeconfigWithInValidClientCert)}
					return nil
				})

				_, err := getCurrentCertificate(context.TODO(), log, mockSeedClient, gardenClientConnection)
				Expect(err).To(HaveOccurred())
			})
		})

		Describe("#getTargetedSeeds", func() {
			It("should return a single Seed", func() {
				mockSeedClient.EXPECT().Get(ctx, kutil.Key(seedName), gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, seed *gardencorev1beta1.Seed) error {
					seed.ObjectMeta = metav1.ObjectMeta{
						Name: seedName,
					}
					return nil
				})

				seeds, err := getTargetedSeeds(context.TODO(), mockSeedClient, nil, seedName)
				Expect(err).ToNot(HaveOccurred())
				Expect(seeds).To(HaveLen(1))
				Expect(seeds[0]).To(Equal(gardencorev1beta1.Seed{
					ObjectMeta: metav1.ObjectMeta{
						Name: seedName,
					},
				}))
			})

			It("should return all Seed matched by seedSelector", func() {
				items := []gardencorev1beta1.Seed{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "seed-one",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "seed-two",
						},
					},
				}

				seedSelector := &metav1.LabelSelector{MatchLabels: map[string]string{
					"seed-kind": "promiscuous",
				}}

				seedLabelSelector, err := metav1.LabelSelectorAsSelector(seedSelector)
				Expect(err).To(BeNil())
				mockSeedClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.SeedList{}), client.MatchingLabelsSelector{Selector: seedLabelSelector}).DoAndReturn(func(_ context.Context, list *gardencorev1beta1.SeedList, _ ...client.ListOption) error {
					*list = gardencorev1beta1.SeedList{Items: items}
					return nil
				})

				seeds, err := getTargetedSeeds(context.TODO(), mockSeedClient, seedSelector, seedName)
				Expect(err).ToNot(HaveOccurred())
				Expect(seeds).To(HaveLen(2))
				Expect(seeds).To(Equal(items))
			})
		})

		Describe("#waitForCertificateRotation", func() {
			var (
				ctx            = context.TODO()
				testKubeconfig string
			)

			BeforeEach(func() {
				// generate kubeconfigs
				validity := 1 * time.Second
				cert := generateCertificate(validity)
				testKubeconfig = fmt.Sprintf(baseKubeconfig, utils.EncodeBase64(cert.CertificatePEM), utils.EncodeBase64(cert.PrivateKeyPEM))

				// mock first secret retrieval
				mockSeedClient.EXPECT().Get(ctx, kutil.Key(gardenClientConnection.KubeconfigSecret.Namespace, gardenClientConnection.KubeconfigSecret.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, secret *corev1.Secret) error {
					secret.ObjectMeta = metav1.ObjectMeta{
						Name:      gardenClientConnection.KubeconfigSecret.Name,
						Namespace: gardenClientConnection.KubeconfigSecret.Namespace,
					}
					secret.Data = map[string][]byte{kubernetes.KubeConfig: []byte(testKubeconfig)}
					return nil
				})

				mockSeedInterface.EXPECT().DirectClient().Return(mockSeedClient).AnyTimes()
			})

			It("should not return an error", func() {
				// mock second secret retrieval - check the validity of the certificate again
				// in this case the secret has not changed
				mockSeedClient.EXPECT().Get(ctx, kutil.Key(gardenClientConnection.KubeconfigSecret.Namespace, gardenClientConnection.KubeconfigSecret.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, secret *corev1.Secret) error {
					secret.ObjectMeta = metav1.ObjectMeta{
						Name:      gardenClientConnection.KubeconfigSecret.Name,
						Namespace: gardenClientConnection.KubeconfigSecret.Namespace,
					}
					secret.Data = map[string][]byte{kubernetes.KubeConfig: []byte(testKubeconfig)}
					return nil
				})

				subject, dnsSANs, ipSANs, _, err := waitForCertificateRotation(context.TODO(), log, mockSeedClient, gardenClientConnection, time.Now)
				Expect(err).ToNot(HaveOccurred())
				Expect(subject).ToNot(BeNil())
				Expect(subject.CommonName).To(Equal(x509CommonName))
				Expect(subject.Organization).To(Equal(x509Organization))
				Expect(dnsSANs).To(Equal(x509DnsNames))
				Expect(ipSANs).To(Equal(x509IpAddresses))
			})

			It("should return an error - simulate changing the certificate while waiting for the certificate rotation deadline", func() {
				// generate new valid kubeconfig with different certificate validity
				validity := 1 * time.Hour
				cert := generateCertificate(validity)
				updated := fmt.Sprintf(baseKubeconfig, utils.EncodeBase64(cert.CertificatePEM), utils.EncodeBase64(cert.PrivateKeyPEM))

				// mock second secret retrieval - check the validity of the certificate again
				// the secret has been updated!
				mockSeedClient.EXPECT().Get(ctx, kutil.Key(gardenClientConnection.KubeconfigSecret.Namespace, gardenClientConnection.KubeconfigSecret.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, secret *corev1.Secret) error {
					secret.ObjectMeta = metav1.ObjectMeta{
						Name:      gardenClientConnection.KubeconfigSecret.Name,
						Namespace: gardenClientConnection.KubeconfigSecret.Namespace,
					}
					secret.Data = map[string][]byte{kubernetes.KubeConfig: []byte(updated)}
					return nil
				})

				_, _, _, _, err := waitForCertificateRotation(context.TODO(), log, mockSeedClient, gardenClientConnection, time.Now)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

const x509CommonName = "gardener.cloud:system:seed:test"

var (
	x509Organization = []string{"gardener.cloud:system:seeds"}
	x509DnsNames     = []string{"my.alternative.apiserver.domain"}
	x509IpAddresses  = []net.IP{net.ParseIP("100.64.0.10").To4()}
)

func generateCertificate(validity time.Duration) secrets.Certificate {
	caCertConfig := &secrets.CertificateSecretConfig{
		Name:         "test",
		CommonName:   x509CommonName,
		Organization: x509Organization,
		DNSNames:     x509DnsNames,
		IPAddresses:  x509IpAddresses,
		CertType:     secrets.ClientCert,
		Validity:     &validity,
	}
	cert, err := caCertConfig.GenerateCertificate()
	Expect(err).ToNot(HaveOccurred())
	return *cert
}
