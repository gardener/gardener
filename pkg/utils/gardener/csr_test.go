// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	certificatesv1 "k8s.io/api/certificates/v1"

	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("csr", func() {
	DescribeTable("#IsSeedClientCert",
		func(x509cr *x509.CertificateRequest, usages []certificatesv1.KeyUsage, expectedStatus bool, expectedReason gomegatypes.GomegaMatcher) {
			status, reason := IsSeedClientCert(x509cr, usages)
			Expect(status).To(Equal(expectedStatus))
			Expect(reason).To(expectedReason)
		},

		Entry("org does not match", &x509.CertificateRequest{}, nil, false, ContainSubstring("organization")),
		Entry("dns names given", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:seed:test", Organization: []string{"gardener.cloud:system:seeds"}}, DNSNames: []string{"foo"}}, nil, false, ContainSubstring("DNSNames")),
		Entry("email addresses given", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:seed:test", Organization: []string{"gardener.cloud:system:seeds"}}, EmailAddresses: []string{"foo"}}, nil, false, ContainSubstring("EmailAddresses")),
		Entry("ip addresses given", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:seed:test", Organization: []string{"gardener.cloud:system:seeds"}}, IPAddresses: []net.IP{{}}}, nil, false, ContainSubstring("IPAddresses")),
		Entry("key usages do not match", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:seed:test", Organization: []string{"gardener.cloud:system:seeds"}}}, nil, false, ContainSubstring("key usages")),
		Entry("common name does not match", &x509.CertificateRequest{Subject: pkix.Name{Organization: []string{"gardener.cloud:system:seeds"}}}, []certificatesv1.KeyUsage{certificatesv1.UsageKeyEncipherment, certificatesv1.UsageDigitalSignature, certificatesv1.UsageClientAuth}, false, ContainSubstring("CommonName")),
		Entry("everything matches", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:seed:test", Organization: []string{"gardener.cloud:system:seeds"}}}, []certificatesv1.KeyUsage{certificatesv1.UsageKeyEncipherment, certificatesv1.UsageDigitalSignature, certificatesv1.UsageClientAuth}, true, Equal("")),
	)

	DescribeTable("#IsShootClientCert",
		func(x509cr *x509.CertificateRequest, usages []certificatesv1.KeyUsage, expectedStatus bool, expectedReason gomegatypes.GomegaMatcher) {
			status, reason := IsShootClientCert(x509cr, usages)
			Expect(status).To(Equal(expectedStatus))
			Expect(reason).To(expectedReason)
		},

		Entry("org does not match", &x509.CertificateRequest{}, nil, false, ContainSubstring("organization")),
		Entry("dns names given", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:shoot:test:shoot", Organization: []string{"gardener.cloud:system:shoots"}}, DNSNames: []string{"foo"}}, nil, false, ContainSubstring("DNSNames")),
		Entry("email addresses given", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:shoot:test:shoot", Organization: []string{"gardener.cloud:system:shoots"}}, EmailAddresses: []string{"foo"}}, nil, false, ContainSubstring("EmailAddresses")),
		Entry("ip addresses given", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:shoot:test:shoot", Organization: []string{"gardener.cloud:system:shoots"}}, IPAddresses: []net.IP{{}}}, nil, false, ContainSubstring("IPAddresses")),
		Entry("key usages do not match", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:shoot:test:shoot", Organization: []string{"gardener.cloud:system:shoots"}}}, nil, false, ContainSubstring("key usages")),
		Entry("common name does not start with prefix", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "invalid:name", Organization: []string{"gardener.cloud:system:shoots"}}}, []certificatesv1.KeyUsage{certificatesv1.UsageKeyEncipherment, certificatesv1.UsageDigitalSignature, certificatesv1.UsageClientAuth}, false, ContainSubstring("must start with")),
		Entry("common name missing namespace", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:shoot:", Organization: []string{"gardener.cloud:system:shoots"}}}, []certificatesv1.KeyUsage{certificatesv1.UsageKeyEncipherment, certificatesv1.UsageDigitalSignature, certificatesv1.UsageClientAuth}, false, ContainSubstring("must be")),
		Entry("common name missing shoot name", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:shoot:test:", Organization: []string{"gardener.cloud:system:shoots"}}}, []certificatesv1.KeyUsage{certificatesv1.UsageKeyEncipherment, certificatesv1.UsageDigitalSignature, certificatesv1.UsageClientAuth}, false, ContainSubstring("must be")),
		Entry("common name with empty namespace", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:shoot::shoot", Organization: []string{"gardener.cloud:system:shoots"}}}, []certificatesv1.KeyUsage{certificatesv1.UsageKeyEncipherment, certificatesv1.UsageDigitalSignature, certificatesv1.UsageClientAuth}, false, ContainSubstring("must be")),
		Entry("common name with too many parts", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:shoot:test:shoot:extra", Organization: []string{"gardener.cloud:system:shoots"}}}, []certificatesv1.KeyUsage{certificatesv1.UsageKeyEncipherment, certificatesv1.UsageDigitalSignature, certificatesv1.UsageClientAuth}, false, ContainSubstring("must be")),
		Entry("everything matches", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:shoot:test:shoot", Organization: []string{"gardener.cloud:system:shoots"}}}, []certificatesv1.KeyUsage{certificatesv1.UsageKeyEncipherment, certificatesv1.UsageDigitalSignature, certificatesv1.UsageClientAuth}, true, Equal("")),
	)
})
