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

package gardener_test

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"net"

	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	certificatesv1 "k8s.io/api/certificates/v1"

	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("utils", func() {
	DescribeTable("#ComputeGardenNamespace",
		func(name, expected string) {
			Expect(ComputeGardenNamespace(name)).To(Equal(expected))
		},

		Entry("empty name", "", "seed-"),
		Entry("garden", "garden", "seed-garden"),
		Entry("dash", "-", "seed--"),
		Entry("garden prefixed with dash", "-garden", "seed--garden"),
	)

	DescribeTable("#ComputeSeedName",
		func(name, expected string) {
			Expect(ComputeSeedName(name)).To(Equal(expected))
		},

		Entry("expect error with empty name", "", ""),
		Entry("expect error with garden name", "garden", ""),
		Entry("expect error with dash", "-", ""),
		Entry("expect success with empty name", "seed-", ""),
		Entry("expect success with dash name", "seed--", "-"),
		Entry("expect success with duplicated prefix", "seed-seed-", "seed-"),
		Entry("expect success with duplicated prefix", "seed-seed-a", "seed-a"),
		Entry("expect success with garden name", "seed-garden", "garden"),
	)

	DescribeTable("#IsSeedClientCert",
		func(x509cr *x509.CertificateRequest, usages []certificatesv1.KeyUsage, expectedStatus bool, expectedReason gomegatypes.GomegaMatcher) {
			status, reason := IsSeedClientCert(x509cr, usages)
			Expect(status).To(Equal(expectedStatus))
			Expect(reason).To(expectedReason)
		},

		Entry("org does not match", &x509.CertificateRequest{}, nil, false, ContainSubstring("organization")),
		Entry("dns names given", &x509.CertificateRequest{Subject: pkix.Name{Organization: []string{"gardener.cloud:system:seeds"}}, DNSNames: []string{"foo"}}, nil, false, ContainSubstring("DNSNames")),
		Entry("email addresses given", &x509.CertificateRequest{Subject: pkix.Name{Organization: []string{"gardener.cloud:system:seeds"}}, EmailAddresses: []string{"foo"}}, nil, false, ContainSubstring("EmailAddresses")),
		Entry("ip addresses given", &x509.CertificateRequest{Subject: pkix.Name{Organization: []string{"gardener.cloud:system:seeds"}}, IPAddresses: []net.IP{{}}}, nil, false, ContainSubstring("IPAddresses")),
		Entry("key usages do not match", &x509.CertificateRequest{Subject: pkix.Name{Organization: []string{"gardener.cloud:system:seeds"}}}, nil, false, ContainSubstring("key usages")),
		Entry("common name does not match", &x509.CertificateRequest{Subject: pkix.Name{Organization: []string{"gardener.cloud:system:seeds"}}}, []certificatesv1.KeyUsage{certificatesv1.UsageKeyEncipherment, certificatesv1.UsageDigitalSignature, certificatesv1.UsageClientAuth}, false, ContainSubstring("CommonName")),
		Entry("everything matches", &x509.CertificateRequest{Subject: pkix.Name{Organization: []string{"gardener.cloud:system:seeds"}, CommonName: "gardener.cloud:system:seed:foo"}}, []certificatesv1.KeyUsage{certificatesv1.UsageKeyEncipherment, certificatesv1.UsageDigitalSignature, certificatesv1.UsageClientAuth}, true, Equal("")),
	)
})
