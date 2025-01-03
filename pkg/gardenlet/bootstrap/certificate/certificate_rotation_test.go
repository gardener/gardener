// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certificate

import (
	"context"
	"crypto/x509/pkix"
	"fmt"
	"net"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/mock"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/kubernetes/certificatesigningrequest"
	"github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/test"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("Certificates", func() {
	var (
		log = logr.Discard()

		ctx       context.Context
		ctxCancel context.CancelFunc

		ctrl                *gomock.Controller
		mockGardenInterface *mock.MockInterface

		mockGardenClient *mockclient.MockClient
		mockSeedClient   *mockclient.MockClient

		gardenClientConnection = &gardenletconfigv1alpha1.GardenClientConnection{
			KubeconfigSecret: &corev1.SecretReference{
				Name:      "gardenlet-kubeconfig",
				Namespace: "garden",
			},
			KubeconfigValidity: &gardenletconfigv1alpha1.KubeconfigValidity{},
		}

		approvedCSR = certificatesv1.CertificateSigningRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name: "approved-csr",
			},
			Status: certificatesv1.CertificateSigningRequestStatus{
				Conditions: []certificatesv1.CertificateSigningRequestCondition{
					{
						Type: certificatesv1.CertificateApproved,
					},
				},
				Certificate: []byte("my-cert"),
			},
		}

		deniedCSR = certificatesv1.CertificateSigningRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name: "denied-csr",
			},
			Status: certificatesv1.CertificateSigningRequestStatus{
				Conditions: []certificatesv1.CertificateSigningRequestCondition{
					{
						Type: certificatesv1.CertificateDenied,
					},
				},
			},
		}

		failedCSR = certificatesv1.CertificateSigningRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name: "failed-csr",
			},
			Status: certificatesv1.CertificateSigningRequestStatus{
				Conditions: []certificatesv1.CertificateSigningRequestCondition{
					{
						Type: certificatesv1.CertificateFailed,
					},
				},
			},
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockSeedClient = mockclient.NewMockClient(ctrl)
		ctx, ctxCancel = context.WithTimeout(context.Background(), 1*time.Minute)
	})

	AfterEach(func() {
		ctrl.Finish()
		ctxCancel()
	})

	Describe("#rotateCertificate", func() {
		var (
			certificateSubject = pkix.Name{CommonName: x509CommonName}
			kubeClient         *fake.Clientset
		)

		BeforeEach(func() {
			mockGardenInterface = mock.NewMockInterface(ctrl)
			mockGardenClient = mockclient.NewMockClient(ctrl)
			mockGardenInterface.EXPECT().Client().Return(mockGardenClient).AnyTimes()

			kubeClient = fake.NewSimpleClientset()
			kubeClient.Fake = testing.Fake{Resources: []*metav1.APIResourceList{
				{
					GroupVersion: "v1",
					APIResources: []metav1.APIResource{
						{
							Name:       "certificatesigningrequests",
							Namespaced: true,
							Group:      certificatesv1.GroupName,
							Version:    certificatesv1.SchemeGroupVersion.Version,
							Kind:       "CertificateSigningRequest",
						},
					},
				},
			}}
		})

		It("should not return an error", func() {
			defer test.WithVar(&certificatesigningrequest.DigestedName, func(any, *pkix.Name, []certificatesv1.KeyUsage, string) (string, error) {
				return approvedCSR.Name, nil
			})()

			kubeClient.AddReactor("*", "certificatesigningrequests", func(_ testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, &approvedCSR, nil
			})
			mockGardenInterface.EXPECT().Kubernetes().Return(kubeClient)

			// mock gardenClient.RESTConfig()
			certClientConfig := &rest.Config{Host: "testhost", TLSClientConfig: rest.TLSClientConfig{
				Insecure: false,
				CAFile:   "filepath",
			}}
			mockGardenInterface.EXPECT().RESTConfig().Return(certClientConfig)

			// mock update of secret in seed with the rotated kubeconfig
			mockSeedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: gardenClientConnection.KubeconfigSecret.Namespace, Name: gardenClientConnection.KubeconfigSecret.Name}, gomock.AssignableToTypeOf(&corev1.Secret{}))
			mockSeedClient.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).
				DoAndReturn(func(_ context.Context, secret *corev1.Secret, _ client.Patch, _ ...client.PatchOption) error {
					Expect(secret.Name).To(Equal(gardenClientConnection.KubeconfigSecret.Name))
					Expect(secret.Namespace).To(Equal(gardenClientConnection.KubeconfigSecret.Namespace))
					Expect(secret.Data).ToNot(BeEmpty())
					return nil
				})

			err := rotateCertificate(ctx, log, mockGardenInterface, mockSeedClient, gardenClientConnection, &certificateSubject, []string{}, []net.IP{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error - CSR is denied", func() {
			defer test.WithVar(&certificatesigningrequest.DigestedName, func(any, *pkix.Name, []certificatesv1.KeyUsage, string) (string, error) {
				return deniedCSR.Name, nil
			})()

			kubeClient.AddReactor("*", "certificatesigningrequests", func(_ testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, &deniedCSR, nil
			})

			mockGardenInterface.EXPECT().Kubernetes().Return(kubeClient)

			err := rotateCertificate(ctx, log, mockGardenInterface, mockSeedClient, gardenClientConnection, &certificateSubject, []string{}, []net.IP{})
			Expect(err).To(MatchError(ContainSubstring("request is denied")))
		})

		It("should return an error - CSR is failed", func() {
			defer test.WithVar(&certificatesigningrequest.DigestedName, func(any, *pkix.Name, []certificatesv1.KeyUsage, string) (string, error) {
				return failedCSR.Name, nil
			})()

			kubeClient.AddReactor("*", "certificatesigningrequests", func(_ testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, &failedCSR, nil
			})

			mockGardenInterface.EXPECT().Kubernetes().Return(kubeClient)

			err := rotateCertificate(ctx, log, mockGardenInterface, mockSeedClient, gardenClientConnection, &certificateSubject, []string{}, []net.IP{})
			Expect(err).To(MatchError(ContainSubstring("request failed")))
		})

		It("should return an error - the CN of the x509 cert to be rotated is not set", func() {
			defer test.WithVar(&certificatesigningrequest.DigestedName, func(any, *pkix.Name, []certificatesv1.KeyUsage, string) (string, error) {
				return deniedCSR.Name, nil
			})()

			kubeClient.AddReactor("*", "certificatesigningrequests", func(_ testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, &deniedCSR, nil
			})

			mockGardenInterface.EXPECT().Kubernetes().Return(kubeClient)

			err := rotateCertificate(ctx, log, mockGardenInterface, mockSeedClient, gardenClientConnection, nil, x509DnsNames, x509IpAddresses)
			Expect(err).To(MatchError(ContainSubstring("The Common Name (CN) of the of the certificate Subject has to be set")))
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

		Describe("#GetCurrentCertificate", func() {
			BeforeEach(func() {
				// generate kubeconfigs
				validity := 20 * time.Second
				cert := generateCertificate(validity)
				gardenKubeconfigWithValidClientCert = fmt.Sprintf(baseKubeconfig, utils.EncodeBase64(cert.CertificatePEM), utils.EncodeBase64(cert.PrivateKeyPEM))
				gardenKubeconfigWithInValidClientCert = fmt.Sprintf(baseKubeconfig, "bm90LXZhbGlk", utils.EncodeBase64(cert.PrivateKeyPEM))
			})

			It("should not return an error", func() {
				cert, err := GetCurrentCertificate(log, []byte(gardenKubeconfigWithValidClientCert), gardenClientConnection)
				Expect(err).ToNot(HaveOccurred())
				Expect(cert).ToNot(BeNil())
			})

			It("should return an error - kubeconfig client cert is invalid", func() {
				_, err := GetCurrentCertificate(log, []byte(gardenKubeconfigWithInValidClientCert), gardenClientConnection)
				Expect(err).To(HaveOccurred())
			})
		})

		Describe("#readCertificateFromKubeconfigSecret", func() {
			It("should return an error - kubeconfig secret does not exist", func() {
				secretGroupResource := schema.GroupResource{Resource: "Secrets"}
				secretNotFoundErr := apierrors.NewNotFound(secretGroupResource, gardenClientConnection.KubeconfigSecret.Name)
				mockSeedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: gardenClientConnection.KubeconfigSecret.Namespace, Name: gardenClientConnection.KubeconfigSecret.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(secretNotFoundErr)

				_, _, err := readCertificateFromKubeconfigSecret(ctx, log, mockSeedClient, gardenClientConnection)
				Expect(err).To(MatchError(ContainSubstring("does not contain a kubeconfig and there is no fallback kubeconfig")))
			})

			It("should return an error - secret does not contain a kubeconfig", func() {
				// mock existing secret with missing garden kubeconfig
				mockSeedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: gardenClientConnection.KubeconfigSecret.Namespace, Name: gardenClientConnection.KubeconfigSecret.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, secret *corev1.Secret, _ ...client.GetOption) error {
					secret.ObjectMeta = metav1.ObjectMeta{
						Name:      gardenClientConnection.KubeconfigSecret.Name,
						Namespace: gardenClientConnection.KubeconfigSecret.Namespace,
					}
					secret.Data = nil
					return nil
				})

				_, _, err := readCertificateFromKubeconfigSecret(ctx, log, mockSeedClient, gardenClientConnection)
				Expect(err).To(MatchError(ContainSubstring("does not contain a kubeconfig and there is no fallback kubeconfig")))
			})
		})

		Describe("#waitForCertificateRotation", func() {
			var (
				testKubeconfig string
			)

			Context("no immediate renewal request", func() {
				BeforeEach(func() {
					// generate kubeconfigs
					validity := time.Millisecond
					cert := generateCertificate(validity)
					testKubeconfig = fmt.Sprintf(baseKubeconfig, utils.EncodeBase64(cert.CertificatePEM), utils.EncodeBase64(cert.PrivateKeyPEM))

					// mock first secret retrieval
					mockSeedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: gardenClientConnection.KubeconfigSecret.Namespace, Name: gardenClientConnection.KubeconfigSecret.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, secret *corev1.Secret, _ ...client.GetOption) error {
						secret.ObjectMeta = metav1.ObjectMeta{
							Name:      gardenClientConnection.KubeconfigSecret.Name,
							Namespace: gardenClientConnection.KubeconfigSecret.Namespace,
						}
						secret.Data = map[string][]byte{kubernetes.KubeConfig: []byte(testKubeconfig)}
						return nil
					})
				})

				It("should not return an error", func() {
					// mock second secret retrieval - check the validity of the certificate again
					// in this case the secret has not changed
					mockSeedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: gardenClientConnection.KubeconfigSecret.Namespace, Name: gardenClientConnection.KubeconfigSecret.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, secret *corev1.Secret, _ ...client.GetOption) error {
						secret.ObjectMeta = metav1.ObjectMeta{
							Name:      gardenClientConnection.KubeconfigSecret.Name,
							Namespace: gardenClientConnection.KubeconfigSecret.Namespace,
						}
						secret.Data = map[string][]byte{kubernetes.KubeConfig: []byte(testKubeconfig)}
						return nil
					})

					subject, dnsSANs, ipSANs, _, err := waitForCertificateRotation(ctx, log, mockSeedClient, gardenClientConnection, time.Now)
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
					mockSeedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: gardenClientConnection.KubeconfigSecret.Namespace, Name: gardenClientConnection.KubeconfigSecret.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, secret *corev1.Secret, _ ...client.GetOption) error {
						secret.ObjectMeta = metav1.ObjectMeta{
							Name:      gardenClientConnection.KubeconfigSecret.Name,
							Namespace: gardenClientConnection.KubeconfigSecret.Namespace,
						}
						secret.Data = map[string][]byte{kubernetes.KubeConfig: []byte(updated)}
						return nil
					})

					_, _, _, _, err := waitForCertificateRotation(ctx, log, mockSeedClient, gardenClientConnection, time.Now)
					Expect(err).To(HaveOccurred())
				})
			})

			It("should return not an error - simulate an immediate renewal request although the current's cert is still valid long enough", func() {
				// generate kubeconfigs
				validity := 500 * time.Hour
				cert := generateCertificate(validity)
				testKubeconfig = fmt.Sprintf(baseKubeconfig, utils.EncodeBase64(cert.CertificatePEM), utils.EncodeBase64(cert.PrivateKeyPEM))

				// mock first secret retrieval - it is annotated with the renew operation - hence, no need to mock
				// second secret retrieval
				mockSeedClient.EXPECT().Get(ctx, client.ObjectKey{Namespace: gardenClientConnection.KubeconfigSecret.Namespace, Name: gardenClientConnection.KubeconfigSecret.Name}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, secret *corev1.Secret, _ ...client.GetOption) error {
					secret.ObjectMeta = metav1.ObjectMeta{
						Name:        gardenClientConnection.KubeconfigSecret.Name,
						Namespace:   gardenClientConnection.KubeconfigSecret.Namespace,
						Annotations: map[string]string{"gardener.cloud/operation": "renew"},
					}
					secret.Data = map[string][]byte{kubernetes.KubeConfig: []byte(testKubeconfig)}
					return nil
				})

				subject, dnsSANs, ipSANs, _, err := waitForCertificateRotation(ctx, log, mockSeedClient, gardenClientConnection, time.Now)
				Expect(err).ToNot(HaveOccurred())
				Expect(subject).ToNot(BeNil())
				Expect(subject.CommonName).To(Equal(x509CommonName))
				Expect(subject.Organization).To(Equal(x509Organization))
				Expect(dnsSANs).To(Equal(x509DnsNames))
				Expect(ipSANs).To(Equal(x509IpAddresses))
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
