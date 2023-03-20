// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seedidentity_test

import (
	"crypto/x509"
	"crypto/x509/pkix"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apiserver/pkg/authentication/user"

	. "github.com/gardener/gardener/pkg/admissioncontroller/seedidentity"
)

var _ = Describe("identity", func() {
	DescribeTable("#FromUserInfoInterface",
		func(u user.Info, expectedSeedName string, expectedIsSeedValue bool) {
			seedName, isSeed := FromUserInfoInterface(u)

			Expect(seedName).To(Equal(expectedSeedName))
			Expect(isSeed).To(Equal(expectedIsSeedValue))
		},

		Entry("nil", nil, "", false),
		Entry("no user name prefix", &user.DefaultInfo{Name: "foo"}, "", false),
		Entry("user name prefix but no groups", &user.DefaultInfo{Name: "gardener.cloud:system:seed:foo"}, "", false),
		Entry("user name prefix but seed group not present", &user.DefaultInfo{Name: "gardener.cloud:system:seed:foo", Groups: []string{"bar"}}, "", false),
		Entry("user name prefix and seed group", &user.DefaultInfo{Name: "gardener.cloud:system:seed:foo", Groups: []string{"gardener.cloud:system:seeds"}}, "foo", true),
	)

	DescribeTable("#FromAuthenticationV1UserInfo",
		func(u authenticationv1.UserInfo, expectedSeedName string, expectedIsSeedValue bool) {
			seedName, isSeed := FromAuthenticationV1UserInfo(u)

			Expect(seedName).To(Equal(expectedSeedName))
			Expect(isSeed).To(Equal(expectedIsSeedValue))
		},

		Entry("no user name prefix", authenticationv1.UserInfo{Username: "foo"}, "", false),
		Entry("user name prefix but no groups", authenticationv1.UserInfo{Username: "gardener.cloud:system:seed:foo"}, "", false),
		Entry("user name prefix but seed group not present", authenticationv1.UserInfo{Username: "gardener.cloud:system:seed:foo", Groups: []string{"bar"}}, "", false),
		Entry("user name prefix and seed group", authenticationv1.UserInfo{Username: "gardener.cloud:system:seed:foo", Groups: []string{"gardener.cloud:system:seeds"}}, "foo", true),
	)

	DescribeTable("#FromCertificateSigningRequest",
		func(csr *x509.CertificateRequest, expectedSeedName string, expectedIsSeedValue bool) {
			seedName, isSeed := FromCertificateSigningRequest(csr)

			Expect(seedName).To(Equal(expectedSeedName))
			Expect(isSeed).To(Equal(expectedIsSeedValue))
		},

		Entry("no user name prefix", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "foo"}}, "", false),
		Entry("user name prefix but no groups", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:seed:foo"}}, "", false),
		Entry("user name prefix but seed group not present", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:seed:foo", Organization: []string{"bar"}}}, "", false),
		Entry("user name prefix and seed group", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:seed:foo", Organization: []string{"gardener.cloud:system:seeds"}}}, "foo", true),
	)
})
