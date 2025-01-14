// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	"crypto/x509"
	"errors"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"golang.org/x/crypto/bcrypt"

	. "github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("Encoding", func() {
	Describe("#CreateBcryptCredentials", func() {
		It("should create the expected credentials", func() {
			credentials, err := CreateBcryptCredentials([]byte("username"), []byte("password"))
			Expect(err).ToNot(HaveOccurred())
			hashedPassword := strings.TrimPrefix(string(credentials), "username:")
			Expect(bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte("password"))).To(Succeed())
		})
	})

	DescribeTable("#ComputeGardenNamespace",
		func(data []byte, csrMatcher func(*x509.CertificateRequest), errMatcher gomegatypes.GomegaMatcher) {
			csr, err := DecodeCertificateRequest(data)
			csrMatcher(csr)
			Expect(err).To(errMatcher)
		},

		Entry("data is nil",
			nil,
			func(csr *x509.CertificateRequest) {
				Expect(csr).To(BeNil())
			},
			MatchError(errors.New("PEM block type must be CERTIFICATE REQUEST")),
		),

		// spellchecker:off
		Entry("data is no CSR",
			[]byte(`-----BEGIN CERTIFICATE-----
MIIDAjCCAeqgAwIBAgIRALm+TCqth9laBtLixvzY0QMwDQYJKoZIhvcNAQELBQAw
GDEWMBQGA1UEAxMNa3ViZXJuZXRlcy1jYTAeFw0yMTA0MjAxMDA5NTlaFw0zMTA0
MjAxMDA5NTlaMCoxKDAmBgNVBAMTH2dhcmRlbmVyLmNsb3VkOnN5c3RlbTpzY2hl
ZHVsZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC9evZPyHGr8ANc
2mKA+hqb5glTv+PXpRD0ms/ZrRmho5IIIFjg3Lg7Zvlj5tgQd7lYhRYDsLudioDj
tm2cc0txNIPpcfly77imfSx1PzGRpHvqZCJMkMBSDsFgEUNp1+Fe6uBydrInC3RF
1AVu0m+yXmrQTuVi8R6Yw7tBA+Ri1Lo6IMUB5o247I1MnDoT3SOjhYEzUAsoBRVC
TE4MG6HY8CxCXnJo4E3Kg86rrEjFOUXDqQsf9MMaLOEHONwGGL/9BOq0nx6CHm06
eQP1w5QgtCSo/0l2K8MynBWdUEPXj/zYTkSzgAlF/2ry7cXxG/r6z5KIGLQ1Pmy2
GFJQFgTtAgMBAAGjNTAzMA4GA1UdDwEB/wQEAwIFoDATBgNVHSUEDDAKBggrBgEF
BQcDAjAMBgNVHRMBAf8EAjAAMA0GCSqGSIb3DQEBCwUAA4IBAQAD8QontES/613+
GIiqTQJIm9FVg+/Co7NRfikeRb4xaakhQih33U4yts4GtRcIu1+dpGQa//M/h2ZA
C6tqNevOV5pamSgxf+BUi8Cy/Aw7tstPmdwUrhPJ++aHdxrVor+gZAWse4MDx2th
eVr+HZ2/OqQWR6GCJvBurvHbKAL/OE6+dOKs/m0RTBguA5mEupEMiVpc8wugtY3P
VwrlW5w5FBRjxqIfVvTPyijJeA3DjooKMNgCq98ghZfaZPLvYAb5RDi4mhJnQwuc
Y4ud3vcGwEsGQx5P8oJ/wanM/Fp4h1QTda1Fim3QkeeVKYu1r4DEeU4ROP7j3hUB
VusoisJW
-----END CERTIFICATE-----`),
			func(csr *x509.CertificateRequest) {
				Expect(csr).To(BeNil())
			},
			MatchError(errors.New("PEM block type must be CERTIFICATE REQUEST")),
		),

		Entry("data is CSR",
			[]byte(`-----BEGIN CERTIFICATE REQUEST-----
MIIClDCCAXwCAQAwTzEkMCIGA1UEChMbZ2FyZGVuZXIuY2xvdWQ6c3lzdGVtOnNl
ZWRzMScwJQYDVQQDEx5nYXJkZW5lci5jbG91ZDpzeXN0ZW06c2VlZDphd3MwggEi
MA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDvXQdCxfhxtcPpcRuZgx6/7+O5
NuE4WkovazMSbpgCx+JFLfdYRdkGMvaF24RIycZOgCR/SXjFy6pufWr48/Rloq1t
efWvYIxzCVw8OYbKvnE7BoR7FyY7z+mn/A5MHcTHraKLoIzEbeCnQJ7B5Xe1ApiJ
aqvyRjulXs7h0W6lyZnJdmAHe6dKvzcdf8XcsLL9ihQUhH5JxIuBorYzLjw3i5DZ
praXAZUHAorFYQHIXESWA3TaOhXieFvX/jspOCRW0w2A+gBgBBrjq3/Yi9Ud738m
NS3s5IGjRip5yplGHBW/SuFXZv6YBAHwXKzxlc4tvr/SI/ZVoZ278T5IBspFAgMB
AAGgADANBgkqhkiG9w0BAQsFAAOCAQEAQhToLaidF7+IlmkXeZuT1S8ORTEdJccD
3k7jagbwifhJItRpBLqUr5d4gLO5mfvfgvhEp8IqB9zaMLOTXXu2w8/qIZU2QpUC
mgr+NYQqXrk/RRVcH6i7nCHbIjeHwm/BeA0hRmJtH1xqkwK/8RCUvq2YNnGIWcjh
1akJDMTTwKjT2RCZjyDaltfoqIjV9o4a/GyM8jds4GryzEtXbSvEpzHPRwHdXCBq
CQo7dCxWxLd19oyUkHxhnY9sEff25xrdcL/R21IOTYFx3OJoYliGCUA1KnZsEpw8
scLaDjVwpKfl9g24hVxCoyVl8Kcuqax5OcXba3h4/dEgq/N2hbpsnQ==
-----END CERTIFICATE REQUEST-----`),
			func(csr *x509.CertificateRequest) {
				Expect(csr.Subject.CommonName).To(Equal("gardener.cloud:system:seed:aws"))
				Expect(csr.Subject.Organization).To(ConsistOf("gardener.cloud:system:seeds"))
			},
			BeNil(),
		),
		// spellchecker:on
	)
})
