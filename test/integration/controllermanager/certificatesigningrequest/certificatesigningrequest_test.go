// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certificatesigningrequest_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509/pkix"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	certificatesv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	certutil "k8s.io/client-go/util/cert"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

var _ = Describe("CSR autoapprove controller tests", func() {
	var (
		csr                *certificatesv1.CertificateSigningRequest
		certificateSubject *pkix.Name
		privateKey         *rsa.PrivateKey
		csrData            []byte
		err                error
	)

	BeforeEach(func() {
		privateKey, _ = secretsutils.FakeGenerateKey(rand.Reader, 4096)

		csr = &certificatesv1.CertificateSigningRequest{
			// Username, UID, Groups will be injected by API server.
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testID + "-",
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: certificatesv1.CertificateSigningRequestSpec{
				Usages: []certificatesv1.KeyUsage{
					certificatesv1.UsageDigitalSignature,
					certificatesv1.UsageKeyEncipherment,
					certificatesv1.UsageClientAuth,
				},
				SignerName: certificatesv1.KubeAPIServerClientSignerName,
			},
		}
	})

	JustBeforeEach(func() {
		By("Create CSR")
		Expect(testClient.Create(ctx, csr)).To(Succeed())
		log.Info("Created CSR for test", "csr", client.ObjectKeyFromObject(csr))

		DeferCleanup(func() {
			By("Delete CSR")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, csr))).To(Succeed())
		})
	})

	Context("non seed client certificate", func() {
		BeforeEach(func() {
			certificateSubject = &pkix.Name{
				CommonName: "csr-autoapprove-test",
			}
			csrData, err = certutil.MakeCSR(privateKey, certificateSubject, nil, nil)
			Expect(err).NotTo(HaveOccurred())
			csr.Spec.Request = csrData
		})

		It("should ignore the CSR and do nothing", func() {
			Consistently(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(csr), csr)).To(Succeed())
				g.Expect(csr.Status.Conditions).To(BeEmpty())
			}).Should(Succeed())
		})
	})

	Context("seed client certificate", func() {
		BeforeEach(func() {
			certificateSubject = &pkix.Name{
				Organization: []string{v1beta1constants.SeedsGroup},
				CommonName:   v1beta1constants.SeedUserNamePrefix + "csr-autoapprove-test",
			}
			csrData, err = certutil.MakeCSR(privateKey, certificateSubject, nil, nil)
			Expect(err).NotTo(HaveOccurred())
			csr.Spec.Request = csrData
		})

		It("should approve the csr", func() {
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(csr), csr)).To(Succeed())
				g.Expect(csr.Status.Conditions).To(ContainElement(And(
					HaveField("Type", certificatesv1.CertificateApproved),
					HaveField("Reason", "AutoApproved"),
					HaveField("Message", "Auto approving gardenlet client certificate after SubjectAccessReview."),
				)))
			}).Should(Succeed())
		})
	})
})
