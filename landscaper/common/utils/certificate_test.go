// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package utils

import (
	"time"

	"github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Utils", func() {
	Describe("#CertificateNeedsRenewal", func() {
		var (
			notBefore time.Time
			validity  time.Duration

			caCert *secrets.Certificate

			validityPercentage float64
		)

		BeforeEach(func() {
			notBefore = getTime("2017-01-01T00:00:00.000Z")
			validity = 10 * time.Second
			validityPercentage = 0.8 // when 80% of the validity is elapsed the certificate should be renewed

			caCert = generateCaCert()
		})

		Context("within validity threshold", func() {
			It("should not require certificate renewal - 80% validity threshold", func() {
				jumpToFuture := 7 * time.Second // within 80% validity threshold
				nowFunc := func() time.Time {
					return notBefore.Add(jumpToFuture)
				}

				defer test.WithVar(&NowFunc, nowFunc)()

				cert := generateClientCert(caCert, notBefore, validity).Certificate
				needsRenewal, _ := CertificateNeedsRenewal(cert, validityPercentage)
				Expect(needsRenewal).To(BeFalse())
			})

			It("should not require certificate renewal - 100% validity threshold", func() {
				validityPercentage = 1           // complete validity range is used - 100%
				jumpToFuture := 10 * time.Second // within 100% validity threshold (validity is also 10s)
				nowFunc := func() time.Time {
					return notBefore.Add(jumpToFuture)
				}

				defer test.WithVar(&NowFunc, nowFunc)()

				cert := generateClientCert(caCert, notBefore, validity).Certificate
				needsRenewal, _ := CertificateNeedsRenewal(cert, validityPercentage)
				Expect(needsRenewal).To(BeFalse())
			})
		})

		Context("not within validity threshold", func() {
			It("should require certificate renewal", func() {
				jumpToFuture := 9 * time.Second // not within 80% validity threshold
				nowFunc := func() time.Time {
					return notBefore.Add(jumpToFuture)
				}

				defer test.WithVar(&NowFunc, nowFunc)()

				cert := generateClientCert(caCert, notBefore, validity).Certificate
				needsRenewal, _ := CertificateNeedsRenewal(cert, validityPercentage)
				Expect(needsRenewal).To(BeTrue())

			})
		})

		Context("not valid certificate", func() {
			It("should require certificate renewal for expired certificate", func() {
				jumpToFuture := validity + 1*time.Second
				nowFunc := func() time.Time {
					return time.Now().Add(jumpToFuture)
				}

				defer test.WithVar(&NowFunc, nowFunc)()

				cert := generateClientCert(caCert, notBefore, validity).Certificate
				needsRenewal, _ := CertificateNeedsRenewal(cert, validityPercentage)
				Expect(needsRenewal).To(BeTrue())
			})

			It("should require certificate renewal for not yet valid certificate", func() {
				notBefore = getTime("2017-01-01T00:00:00.000Z")
				now := getTime("2016-01-01T00:00:00.000Z")

				nowFunc := func() time.Time {
					return now
				}

				defer test.WithVar(&NowFunc, nowFunc)()

				cert := generateClientCert(caCert, notBefore, validity).Certificate
				needsRenewal, _ := CertificateNeedsRenewal(cert, validityPercentage)
				Expect(needsRenewal).To(BeTrue())
			})
		})
	})
})

func generateClientCert(caCert *secrets.Certificate, notBefore time.Time, validity time.Duration) *secrets.Certificate {
	csc := &secrets.CertificateSecretConfig{
		Name:       "foo",
		CommonName: "foo",
		CertType:   secrets.ClientCert,
		Validity:   &validity,
		SigningCA:  caCert,
		Now: func() time.Time {
			return notBefore
		},
	}
	cert, err := csc.GenerateCertificate()
	Expect(err).ToNot(HaveOccurred())

	return cert
}

func generateCaCert() *secrets.Certificate {
	csc := &secrets.CertificateSecretConfig{
		Name:       "ca-test",
		CommonName: "ca-test",
		CertType:   secrets.CACert,
	}
	caCertificate, err := csc.GenerateCertificate()
	Expect(err).ToNot(HaveOccurred())

	return caCertificate
}

func getTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	Expect(err).ToNot(HaveOccurred())

	return t
}
