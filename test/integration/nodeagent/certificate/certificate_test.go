// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certificate_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesclientset "k8s.io/client-go/kubernetes"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/nodeagent"
)

var _ = Describe("RequestAndStoreKubeconfig", func() {
	var (
		fs afero.Afero
	)

	BeforeEach(func() {
		fs = afero.Afero{Fs: afero.NewMemMapFs()}

		DeferCleanup(func(ctx context.Context) {
			Expect(testClient.DeleteAllOf(ctx, &certificatesv1.CertificateSigningRequest{})).To(Succeed())
		})
	})

	It("should request a kubeconfig and store it on disk", func(ctx context.Context) {
		go func(ctx context.Context) {
			defer GinkgoRecover()
			Eventually(func(g Gomega) {
				handleCSR(ctx, g, certificatesv1.CertificateApproved)
			}).Should(Succeed())
		}(ctx)

		Expect(nodeagent.RequestAndStoreKubeconfig(ctx, log, fs, nodeAgentUser.Config(), machineName)).To(Succeed())
		kubeconfig, err := fs.ReadFile("/var/lib/gardener-node-agent/credentials/kubeconfig")
		Expect(err).NotTo(HaveOccurred())
		_, err = kubernetes.RESTConfigFromKubeconfig(kubeconfig)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should handle a denied CSR", func(ctx context.Context) {
		go func(ctx context.Context) {
			defer GinkgoRecover()
			Eventually(func(g Gomega) {
				handleCSR(ctx, g, certificatesv1.CertificateDenied)
			}).Should(Succeed())
		}(ctx)

		Expect(nodeagent.RequestAndStoreKubeconfig(ctx, log, fs, nodeAgentUser.Config(), machineName)).To(MatchError(ContainSubstring("is denied")))
	})

	It("should handle a failed CSR", func(ctx context.Context) {
		go func(ctx context.Context) {
			defer GinkgoRecover()
			Eventually(func(g Gomega) {
				handleCSR(ctx, g, certificatesv1.CertificateFailed)
			}).Should(Succeed())
		}(ctx)

		Expect(nodeagent.RequestAndStoreKubeconfig(ctx, log, fs, nodeAgentUser.Config(), machineName)).To(MatchError(ContainSubstring("failed")))
	})
})

func handleCSR(ctx context.Context, g Gomega, conditionType certificatesv1.RequestConditionType) {
	clientSet, err := kubernetesclientset.NewForConfig(restConfig)
	g.Expect(err).NotTo(HaveOccurred())
	csrList, err := clientSet.CertificatesV1().CertificateSigningRequests().List(ctx, metav1.ListOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(csrList.Items).To(HaveLen(1))
	csr := &csrList.Items[0]
	g.Expect(strings.HasPrefix(csr.Name, "node-agent-csr-")).To(BeTrue())
	g.Expect(csr.Spec.SignerName).To(Equal(certificatesv1.KubeAPIServerClientSignerName))
	g.Expect(csr.Spec.Usages).To(Equal(
		[]certificatesv1.KeyUsage{
			certificatesv1.UsageDigitalSignature,
			certificatesv1.UsageKeyEncipherment,
			certificatesv1.UsageClientAuth,
		}))

	timeNow := metav1.Now()
	condition := certificatesv1.CertificateSigningRequestCondition{
		Type:               conditionType,
		Status:             corev1.ConditionTrue,
		Reason:             string(conditionType),
		Message:            string(conditionType),
		LastTransitionTime: timeNow,
		LastUpdateTime:     timeNow,
	}
	csr.Status.Conditions = append(csr.Status.Conditions, condition)
	csr, err = clientSet.CertificatesV1().CertificateSigningRequests().UpdateApproval(ctx, csr.Name, csr, metav1.UpdateOptions{})
	g.Expect(err).NotTo(HaveOccurred())

	if conditionType != certificatesv1.CertificateApproved {
		return
	}

	csr.Status.Certificate = createCertificate(g)
	_, err = clientSet.CertificatesV1().CertificateSigningRequests().UpdateStatus(ctx, csr, metav1.UpdateOptions{})
	if err != nil {
		log.Error(err, "Failed to update CSR")
	}
	g.Expect(err).NotTo(HaveOccurred())
}

func createCertificate(g Gomega) []byte {
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	g.Expect(err).NotTo(HaveOccurred())

	certificateTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Foo Organization"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(30 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, &certificateTemplate, &certificateTemplate, &privateKey.PublicKey, privateKey)
	g.Expect(err).NotTo(HaveOccurred())

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
}
