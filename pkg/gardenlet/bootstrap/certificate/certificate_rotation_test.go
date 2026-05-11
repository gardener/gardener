// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/kubernetes/certificatesigningrequest"
	"github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Certificates", func() {
	var (
		log = logr.Discard()

		ctx       context.Context
		ctxCancel context.CancelFunc

		ctrl                *gomock.Controller
		mockGardenInterface *mock.MockInterface

		fakeClient client.Client

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
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
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
			mockGardenInterface.EXPECT().Client().Return(fakeClient).AnyTimes()

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

			// Create the kubeconfig secret in the fake client (empty, will be patched)
			Expect(fakeClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gardenClientConnection.KubeconfigSecret.Name,
					Namespace: gardenClientConnection.KubeconfigSecret.Namespace,
				},
			})).To(Succeed())

			err := rotateCertificate(ctx, log, mockGardenInterface, fakeClient, gardenClientConnection, &certificateSubject, []string{}, []net.IP{})
			Expect(err).ToNot(HaveOccurred())

			// Verify the secret was patched with kubeconfig data
			secret := &corev1.Secret{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{
				Namespace: gardenClientConnection.KubeconfigSecret.Namespace,
				Name:      gardenClientConnection.KubeconfigSecret.Name,
			}, secret)).To(Succeed())
			Expect(secret.Data).ToNot(BeEmpty())
		})

		It("should return an error - CSR is denied", func() {
			defer test.WithVar(&certificatesigningrequest.DigestedName, func(any, *pkix.Name, []certificatesv1.KeyUsage, string) (string, error) {
				return deniedCSR.Name, nil
			})()

			kubeClient.AddReactor("*", "certificatesigningrequests", func(_ testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, &deniedCSR, nil
			})

			mockGardenInterface.EXPECT().Kubernetes().Return(kubeClient)

			err := rotateCertificate(ctx, log, mockGardenInterface, fakeClient, gardenClientConnection, &certificateSubject, []string{}, []net.IP{})
			Expect(err).To(MatchError(ContainSubstring("is denied")))
		})

		It("should return an error - CSR is failed", func() {
			defer test.WithVar(&certificatesigningrequest.DigestedName, func(any, *pkix.Name, []certificatesv1.KeyUsage, string) (string, error) {
				return failedCSR.Name, nil
			})()

			kubeClient.AddReactor("*", "certificatesigningrequests", func(_ testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, &failedCSR, nil
			})

			mockGardenInterface.EXPECT().Kubernetes().Return(kubeClient)

			err := rotateCertificate(ctx, log, mockGardenInterface, fakeClient, gardenClientConnection, &certificateSubject, []string{}, []net.IP{})
			Expect(err).To(MatchError(ContainSubstring("failed")))
		})

		It("should return an error - the CN of the x509 cert to be rotated is not set", func() {
			defer test.WithVar(&certificatesigningrequest.DigestedName, func(any, *pkix.Name, []certificatesv1.KeyUsage, string) (string, error) {
				return deniedCSR.Name, nil
			})()

			kubeClient.AddReactor("*", "certificatesigningrequests", func(_ testing.Action) (handled bool, ret runtime.Object, err error) {
				return true, &deniedCSR, nil
			})

			mockGardenInterface.EXPECT().Kubernetes().Return(kubeClient)

			err := rotateCertificate(ctx, log, mockGardenInterface, fakeClient, gardenClientConnection, nil, x509DnsNames, x509IpAddresses)
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
				// Secret not created in fakeClient - will return NotFound
				_, _, err := readCertificateFromKubeconfigSecret(ctx, log, fakeClient, gardenClientConnection)
				Expect(err).To(MatchError(ContainSubstring("does not contain a kubeconfig and there is no fallback kubeconfig")))
			})

			It("should return an error - secret does not contain a kubeconfig", func() {
				// Create existing secret with missing garden kubeconfig
				Expect(fakeClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      gardenClientConnection.KubeconfigSecret.Name,
						Namespace: gardenClientConnection.KubeconfigSecret.Namespace,
					},
				})).To(Succeed())

				_, _, err := readCertificateFromKubeconfigSecret(ctx, log, fakeClient, gardenClientConnection)
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
				})

				It("should not return an error", func() {
					Expect(fakeClient.Create(ctx, &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      gardenClientConnection.KubeconfigSecret.Name,
							Namespace: gardenClientConnection.KubeconfigSecret.Namespace,
						},
						Data: map[string][]byte{kubernetes.KubeConfig: []byte(testKubeconfig)},
					})).To(Succeed())

					subject, dnsSANs, ipSANs, _, err := waitForCertificateRotation(ctx, log, fakeClient, gardenClientConnection, time.Now)
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

					// First Get returns testKubeconfig (short-lived), second returns updated (long-lived)
					callCount := 0
					seedClient := fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).WithInterceptorFuncs(interceptor.Funcs{
						Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
							if secret, ok := obj.(*corev1.Secret); ok && key.Name == gardenClientConnection.KubeconfigSecret.Name {
								callCount++
								secret.ObjectMeta = metav1.ObjectMeta{
									Name:      gardenClientConnection.KubeconfigSecret.Name,
									Namespace: gardenClientConnection.KubeconfigSecret.Namespace,
								}
								if callCount == 1 {
									secret.Data = map[string][]byte{kubernetes.KubeConfig: []byte(testKubeconfig)}
								} else {
									secret.Data = map[string][]byte{kubernetes.KubeConfig: []byte(updated)}
								}
								return nil
							}
							return c.Get(ctx, key, obj, opts...)
						},
					}).Build()

					_, _, _, _, err := waitForCertificateRotation(ctx, log, seedClient, gardenClientConnection, time.Now)
					Expect(err).To(HaveOccurred())
				})
			})

			It("should return not an error - simulate an immediate renewal request although the current's cert is still valid long enough", func() {
				// generate kubeconfigs
				validity := 500 * time.Hour
				cert := generateCertificate(validity)
				testKubeconfig = fmt.Sprintf(baseKubeconfig, utils.EncodeBase64(cert.CertificatePEM), utils.EncodeBase64(cert.PrivateKeyPEM))

				Expect(fakeClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:        gardenClientConnection.KubeconfigSecret.Name,
						Namespace:   gardenClientConnection.KubeconfigSecret.Namespace,
						Annotations: map[string]string{"gardener.cloud/operation": "renew"},
					},
					Data: map[string][]byte{kubernetes.KubeConfig: []byte(testKubeconfig)},
				})).To(Succeed())

				subject, dnsSANs, ipSANs, _, err := waitForCertificateRotation(ctx, log, fakeClient, gardenClientConnection, time.Now)
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
