// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certificates_test

import (
	"crypto/x509"
	"net"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/extensions/pkg/webhook"
	. "github.com/gardener/gardener/extensions/pkg/webhook/certificates"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("Certificates", func() {
	Describe("#GenerateUnmanagedCertificates", func() {
		var (
			certDir      string
			providerName = "provider-test"
		)

		BeforeEach(func() {
			certDir = GinkgoT().TempDir()
		})

		DescribeTable("should generate the expected certificate",
			func(mode, url string, assertServerCertFn func(*x509.Certificate)) {
				By("Validate generated CA certificate")
				caCertPEM, err := GenerateUnmanagedCertificates(providerName, certDir, mode, url)
				Expect(err).NotTo(HaveOccurred())

				caCert, err := utils.DecodeCertificate(caCertPEM)
				Expect(err).NotTo(HaveOccurred())
				Expect(caCert.Subject.CommonName).To(Equal(providerName))
				Expect(time.Until(caCert.NotAfter)).To(And(BeNumerically(">", 9*365*24*time.Hour)))

				By("Validate generated server certificate")
				serverCertPEM, err := os.ReadFile(filepath.Join(certDir, "tls.crt"))
				Expect(err).NotTo(HaveOccurred())
				serverCert, err := utils.DecodeCertificate(serverCertPEM)
				Expect(err).NotTo(HaveOccurred())
				Expect(serverCert.Subject.CommonName).To(Equal(providerName))
				assertServerCertFn(serverCert)

				By("Validate generated server key")
				serverKeyPEM, err := os.ReadFile(filepath.Join(certDir, "tls.key"))
				Expect(err).NotTo(HaveOccurred())
				_, err = utils.DecodePrivateKey(serverKeyPEM)
				Expect(err).NotTo(HaveOccurred())
			},

			Entry("url mode; url is '127.0.1.1'", webhook.ModeURL, "127.0.1.1", func(serverCert *x509.Certificate) {
				Expect(serverCert.IPAddresses).To(ConsistOf([]net.IP{ipv4(127, 0, 1, 1)}))
				Expect(serverCert.DNSNames).To(BeEmpty())
			}),
			Entry("url mode; url is '::1'", webhook.ModeURL, "::1", func(serverCert *x509.Certificate) {
				Expect(serverCert.IPAddresses).To(ConsistOf([]net.IP{net.ParseIP("::1")}))
				Expect(serverCert.DNSNames).To(BeEmpty())
			}),
			Entry("url mode; url is 'test.invalid'", webhook.ModeURL, "test.invalid", func(serverCert *x509.Certificate) {
				Expect(serverCert.IPAddresses).To(BeEmpty())
				Expect(serverCert.DNSNames).To(ConsistOf("test.invalid"))
			}),
			Entry("url mode; url is 'test.invalid:8443'", webhook.ModeURL, "test.invalid:8443", func(serverCert *x509.Certificate) {
				Expect(serverCert.IPAddresses).To(BeEmpty())
				Expect(serverCert.DNSNames).To(ConsistOf("test.invalid"))
			}),
			Entry("url mode; url is 'test.invalid:8443:invalid'", webhook.ModeURL, "test.invalid:8443:invalid", func(serverCert *x509.Certificate) {
				Expect(serverCert.IPAddresses).To(BeEmpty())
				Expect(serverCert.DNSNames).To(ConsistOf("test.invalid:8443:invalid"))
			}),
			Entry("service mode", webhook.ModeService, "", func(serverCert *x509.Certificate) {
				Expect(serverCert.IPAddresses).To(BeEmpty())
				Expect(serverCert.DNSNames).To(ConsistOf("gardener-extension-" + providerName))
			}),
		)
	})
})

func ipv4(a, b, c, d byte) net.IP {
	return net.IP{a, b, c, d}
}
