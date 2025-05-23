// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certificatesigningrequest_test

import (
	"context"
	"crypto"
	"crypto/x509/pkix"
	"net"
	"strings"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/util/keyutil"

	. "github.com/gardener/gardener/pkg/utils/kubernetes/certificatesigningrequest"
)

var _ = Describe("Certificate", func() {
	var (
		clientSet kubernetesclientset.Interface
		log       logr.Logger

		certificateSubject *pkix.Name
		dnsSANs            []string
		ipSANs             []net.IP
		validityDuration   *metav1.Duration
		csrPrefix          string

		expectedCertData []byte
	)

	BeforeEach(func() {
		clientSet = fake.NewSimpleClientset()
		log = logr.Discard()

		certificateSubject = &pkix.Name{
			Organization: []string{"foo:org"},
			CommonName:   "foo:name",
		}
		dnsSANs = []string{"foo.local"}
		ipSANs = []net.IP{net.ParseIP("10.0.0.1")}
		validityDuration = &metav1.Duration{Duration: time.Hour}
		csrPrefix = "foo-csr-"

		expectedCertData = []byte("foo-cert")
	})

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
			digest, err := DigestedName(signer.Public(), subject, []certificatesv1.KeyUsage{certificatesv1.UsageDigitalSignature}, "seed-csr-")
			Expect(err).ToNot(HaveOccurred())
			Expect(strings.HasPrefix(digest, "seed-csr-")).To(BeTrue())
		})

		It("should return an error because the public key cannot be marshalled", func() {
			_, err := DigestedName([]byte("test"), nil, []certificatesv1.KeyUsage{certificatesv1.UsageDigitalSignature}, "seed-csr-")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#RequestCertificate", func() {
		It("should return the issued certificate", func(ctx context.Context) {
			go func(ctx context.Context) {
				defer GinkgoRecover()
				Eventually(func(g Gomega) {
					timeNow := metav1.Now()
					condition := certificatesv1.CertificateSigningRequestCondition{
						Type:               certificatesv1.CertificateApproved,
						Status:             corev1.ConditionTrue,
						Reason:             "RequestApproved",
						LastTransitionTime: timeNow,
						LastUpdateTime:     timeNow,
					}
					handleCSR(ctx, g, clientSet, csrPrefix, condition, expectedCertData)
				}).Should(Succeed())
			}(ctx)

			certData, privateKeyData, csrName, err := RequestCertificate(ctx, log, clientSet, certificateSubject, dnsSANs, ipSANs, validityDuration, csrPrefix)
			Expect(err).NotTo(HaveOccurred())
			Expect(certData).To(Equal(expectedCertData))
			Expect(privateKeyData).ToNot(BeEmpty())
			Expect(strings.HasPrefix(csrName, csrPrefix)).To(BeTrue())
		}, NodeTimeout(time.Second*5))

		It("should return an error if the CSR was denied", func(ctx context.Context) {
			go func(ctx context.Context) {
				defer GinkgoRecover()
				Eventually(func(g Gomega) {
					timeNow := metav1.Now()
					condition := certificatesv1.CertificateSigningRequestCondition{
						Type:               certificatesv1.CertificateDenied,
						Status:             corev1.ConditionTrue,
						Reason:             "RequestDenied",
						LastTransitionTime: timeNow,
						LastUpdateTime:     timeNow,
					}
					handleCSR(ctx, g, clientSet, csrPrefix, condition, nil)
				}).Should(Succeed())
			}(ctx)

			_, _, _, err := RequestCertificate(ctx, log, clientSet, certificateSubject, dnsSANs, ipSANs, validityDuration, csrPrefix)
			Expect(err).To(MatchError(ContainSubstring("is denied")))
		}, NodeTimeout(time.Second*5))

		It("should return an error if the CSR failed", func(ctx context.Context) {
			go func(ctx context.Context) {
				defer GinkgoRecover()
				Eventually(func(g Gomega) {
					timeNow := metav1.Now()
					condition := certificatesv1.CertificateSigningRequestCondition{
						Type:               certificatesv1.CertificateFailed,
						Status:             corev1.ConditionTrue,
						Reason:             "RequestFailed",
						LastTransitionTime: timeNow,
						LastUpdateTime:     timeNow,
					}
					handleCSR(ctx, g, clientSet, csrPrefix, condition, nil)
				}).Should(Succeed())
			}(ctx)

			_, _, _, err := RequestCertificate(ctx, log, clientSet, certificateSubject, dnsSANs, ipSANs, validityDuration, csrPrefix)
			Expect(err).To(MatchError(ContainSubstring("failed")))
		}, NodeTimeout(time.Second*5))
	})
})

func handleCSR(ctx context.Context, g Gomega, clientSet kubernetesclientset.Interface, csrPrefix string, condition certificatesv1.CertificateSigningRequestCondition, certData []byte) {
	csrList, err := clientSet.CertificatesV1().CertificateSigningRequests().List(ctx, metav1.ListOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(csrList.Items).To(HaveLen(1))
	csr := csrList.Items[0]
	g.Expect(strings.HasPrefix(csr.Name, csrPrefix)).To(BeTrue())
	g.Expect(csr.Spec.SignerName).To(Equal(certificatesv1.KubeAPIServerClientSignerName))
	g.Expect(csr.Spec.Usages).To(Equal(
		[]certificatesv1.KeyUsage{
			certificatesv1.UsageDigitalSignature,
			certificatesv1.UsageKeyEncipherment,
			certificatesv1.UsageClientAuth,
		}))
	csr.Status.Conditions = append(csr.Status.Conditions, condition)
	csr.Status.Certificate = certData
	_, err = clientSet.CertificatesV1().CertificateSigningRequests().Update(ctx, &csr, metav1.UpdateOptions{})
	g.Expect(err).NotTo(HaveOccurred())
}
