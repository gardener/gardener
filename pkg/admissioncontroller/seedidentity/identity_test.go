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
	Describe("#FromUserInfoInterface", func() {
		test := func(u user.Info, expectedSeedName string, expectedIsSeedValue bool, expectedUserType UserType) {
			seedName, isSeed, userType := FromUserInfoInterface(u)

			Expect(seedName).To(Equal(expectedSeedName))
			Expect(isSeed).To(Equal(expectedIsSeedValue))
			Expect(userType).To(Equal(expectedUserType))
		}

		It("nil", func() {
			test(nil, "", false, "")
		})
		It("no user name prefix", func() {
			test(&user.DefaultInfo{Name: "foo"}, "", false, "")
		})
		It("user name prefix but no groups", func() {
			test(&user.DefaultInfo{Name: "gardener.cloud:system:seed:foo"}, "", false, "")
		})
		It("user name prefix but seed group not present", func() {
			test(&user.DefaultInfo{Name: "gardener.cloud:system:seed:foo", Groups: []string{"bar"}}, "", false, "")
		})
		It("user name prefix and seed group", func() {
			test(&user.DefaultInfo{Name: "gardener.cloud:system:seed:foo", Groups: []string{"gardener.cloud:system:seeds"}}, "foo", true, UserTypeGardenlet)
		})

		It("ServiceAccount without groups", func() {
			test(&user.DefaultInfo{Name: "system:serviceaccount:foo:bar"}, "", false, "")
		})
		It("ServiceAccount without namespace group", func() {
			test(&user.DefaultInfo{Name: "system:serviceaccount:foo:bar", Groups: []string{"system:serviceaccounts"}}, "", false, "")
		})
		It("ServiceAccount in non-seed namespace", func() {
			test(&user.DefaultInfo{Name: "system:serviceaccount:foo:bar", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:foo"}}, "", false, "")
		})
		It("Non-extension ServiceAccount in seed namespace", func() {
			test(&user.DefaultInfo{Name: "system:serviceaccount:seed-foo:bar", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:seed-foo"}}, "", false, "")
		})
		It("Extension ServiceAccount in seed namespace", func() {
			test(&user.DefaultInfo{Name: "system:serviceaccount:seed-foo:extension-bar", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:seed-foo"}}, "foo", true, UserTypeExtension)
		})
	})

	Describe("#FromAuthenticationV1UserInfo", func() {
		test := func(u authenticationv1.UserInfo, expectedSeedName string, expectedIsSeedValue bool, expectedUserType UserType) {
			seedName, isSeed, userType := FromAuthenticationV1UserInfo(u)

			Expect(seedName).To(Equal(expectedSeedName))
			Expect(isSeed).To(Equal(expectedIsSeedValue))
			Expect(userType).To(Equal(expectedUserType))
		}

		It("no user name prefix", func() {
			test(authenticationv1.UserInfo{Username: "foo"}, "", false, "")
		})
		It("user name prefix but no groups", func() {
			test(authenticationv1.UserInfo{Username: "gardener.cloud:system:seed:foo"}, "", false, "")
		})
		It("user name prefix but seed group not present", func() {
			test(authenticationv1.UserInfo{Username: "gardener.cloud:system:seed:foo", Groups: []string{"bar"}}, "", false, "")
		})
		It("user name prefix and seed group", func() {
			test(authenticationv1.UserInfo{Username: "gardener.cloud:system:seed:foo", Groups: []string{"gardener.cloud:system:seeds"}}, "foo", true, UserTypeGardenlet)
		})

		It("ServiceAccount without groups", func() {
			test(authenticationv1.UserInfo{Username: "system:serviceaccount:foo:bar"}, "", false, "")
		})
		It("ServiceAccount without namespace group", func() {
			test(authenticationv1.UserInfo{Username: "system:serviceaccount:foo:bar", Groups: []string{"system:serviceaccounts"}}, "", false, "")
		})
		It("ServiceAccount in non-seed namespace", func() {
			test(authenticationv1.UserInfo{Username: "system:serviceaccount:foo:bar", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:foo"}}, "", false, "")
		})
		It("Non-extension ServiceAccount in seed namespace", func() {
			test(authenticationv1.UserInfo{Username: "system:serviceaccount:seed-foo:bar", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:seed-foo"}}, "", false, "")
		})
		It("Extension ServiceAccount in seed namespace", func() {
			test(authenticationv1.UserInfo{Username: "system:serviceaccount:seed-foo:extension-bar", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:seed-foo"}}, "foo", true, UserTypeExtension)
		})
	})

	Describe("#FromCertificateSigningRequest", func() {
		test := func(csr *x509.CertificateRequest, expectedSeedName string, expectedIsSeedValue bool, expectedUserType UserType) {
			seedName, isSeed, userType := FromCertificateSigningRequest(csr)

			Expect(seedName).To(Equal(expectedSeedName))
			Expect(isSeed).To(Equal(expectedIsSeedValue))
			Expect(userType).To(Equal(expectedUserType))
		}

		It("no user name prefix", func() {
			test(&x509.CertificateRequest{Subject: pkix.Name{CommonName: "foo"}}, "", false, "")
		})
		It("user name prefix but no groups", func() {
			test(&x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:seed:foo"}}, "", false, "")
		})
		It("user name prefix but seed group not present", func() {
			test(&x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:seed:foo", Organization: []string{"bar"}}}, "", false, "")
		})
		It("user name prefix and seed group", func() {
			test(&x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:seed:foo", Organization: []string{"gardener.cloud:system:seeds"}}}, "foo", true, UserTypeGardenlet)
		})
	})
})
