// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed_test

import (
	"crypto/x509"
	"crypto/x509/pkix"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apiserver/pkg/authentication/user"

	"github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity"
	. "github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity/seed"
)

var _ = Describe("identity", func() {
	DescribeTable("#FromUserInfoInterface",
		func(u user.Info, expectedSeedName string, expectedIsSeedValue bool, expectedUserType gardenletidentity.UserType) {
			seedName, isSeed, userType := FromUserInfoInterface(u)

			Expect(seedName).To(Equal(expectedSeedName))
			Expect(isSeed).To(Equal(expectedIsSeedValue))
			Expect(userType).To(Equal(expectedUserType))
		},

		Entry("nil", nil, "", false, gardenletidentity.UserType("")),
		Entry("no user name prefix", &user.DefaultInfo{Name: "foo"}, "", false, gardenletidentity.UserType("")),
		Entry("user name prefix but no groups", &user.DefaultInfo{Name: "gardener.cloud:system:seed:foo"}, "", false, gardenletidentity.UserType("")),
		Entry("user name prefix but seed group not present", &user.DefaultInfo{Name: "gardener.cloud:system:seed:foo", Groups: []string{"bar"}}, "", false, gardenletidentity.UserType("")),
		Entry("user name prefix and seed group", &user.DefaultInfo{Name: "gardener.cloud:system:seed:foo", Groups: []string{"gardener.cloud:system:seeds"}}, "foo", true, gardenletidentity.UserTypeGardenlet),
		Entry("ServiceAccount without groups", &user.DefaultInfo{Name: "system:serviceaccount:foo:bar"}, "", false, gardenletidentity.UserType("")),
		Entry("ServiceAccount without namespace group", &user.DefaultInfo{Name: "system:serviceaccount:foo:bar", Groups: []string{"system:serviceaccounts"}}, "", false, gardenletidentity.UserType("")),
		Entry("ServiceAccount in non-seed namespace", &user.DefaultInfo{Name: "system:serviceaccount:foo:bar", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:foo"}}, "", false, gardenletidentity.UserType("")),
		Entry("Non-extension ServiceAccount in seed namespace", &user.DefaultInfo{Name: "system:serviceaccount:seed-foo:bar", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:seed-foo"}}, "", false, gardenletidentity.UserType("")),
		Entry("Extension ServiceAccount in seed namespace", &user.DefaultInfo{Name: "system:serviceaccount:seed-foo:extension-bar", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:seed-foo"}}, "foo", true, gardenletidentity.UserTypeExtension),
	)

	DescribeTable("#FromAuthenticationV1UserInfo",
		func(u authenticationv1.UserInfo, expectedSeedName string, expectedIsSeedValue bool, expectedUserType gardenletidentity.UserType) {
			seedName, isSeed, userType := FromAuthenticationV1UserInfo(u)

			Expect(seedName).To(Equal(expectedSeedName))
			Expect(isSeed).To(Equal(expectedIsSeedValue))
			Expect(userType).To(Equal(expectedUserType))
		},

		Entry("no user name prefix", authenticationv1.UserInfo{Username: "foo"}, "", false, gardenletidentity.UserType("")),
		Entry("user name prefix but no groups", authenticationv1.UserInfo{Username: "gardener.cloud:system:seed:foo"}, "", false, gardenletidentity.UserType("")),
		Entry("user name prefix but seed group not present", authenticationv1.UserInfo{Username: "gardener.cloud:system:seed:foo", Groups: []string{"bar"}}, "", false, gardenletidentity.UserType("")),
		Entry("user name prefix and seed group", authenticationv1.UserInfo{Username: "gardener.cloud:system:seed:foo", Groups: []string{"gardener.cloud:system:seeds"}}, "foo", true, gardenletidentity.UserTypeGardenlet),
		Entry("ServiceAccount without groups", authenticationv1.UserInfo{Username: "system:serviceaccount:foo:bar"}, "", false, gardenletidentity.UserType("")),
		Entry("ServiceAccount without namespace group", authenticationv1.UserInfo{Username: "system:serviceaccount:foo:bar", Groups: []string{"system:serviceaccounts"}}, "", false, gardenletidentity.UserType("")),
		Entry("ServiceAccount in non-seed namespace", authenticationv1.UserInfo{Username: "system:serviceaccount:foo:bar", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:foo"}}, "", false, gardenletidentity.UserType("")),
		Entry("Non-extension ServiceAccount in seed namespace", authenticationv1.UserInfo{Username: "system:serviceaccount:seed-foo:bar", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:seed-foo"}}, "", false, gardenletidentity.UserType("")),
		Entry("Extension ServiceAccount in seed namespace", authenticationv1.UserInfo{Username: "system:serviceaccount:seed-foo:extension-bar", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:seed-foo"}, Extra: map[string]authenticationv1.ExtraValue{}}, "foo", true, gardenletidentity.UserTypeExtension),
	)

	DescribeTable("#FromCertificateSigningRequest",
		func(csr *x509.CertificateRequest, expectedSeedName string, expectedIsSeedValue bool, expectedUserType gardenletidentity.UserType) {
			seedName, isSeed, userType := FromCertificateSigningRequest(csr)

			Expect(seedName).To(Equal(expectedSeedName))
			Expect(isSeed).To(Equal(expectedIsSeedValue))
			Expect(userType).To(Equal(expectedUserType))
		},

		Entry("no user name prefix", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "foo"}}, "", false, gardenletidentity.UserType("")),
		Entry("user name prefix but no groups", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:seed:foo"}}, "", false, gardenletidentity.UserType("")),
		Entry("user name prefix but seed group not present", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:seed:foo", Organization: []string{"bar"}}}, "", false, gardenletidentity.UserType("")),
		Entry("user name prefix and seed group", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:seed:foo", Organization: []string{"gardener.cloud:system:seeds"}}}, "foo", true, gardenletidentity.UserTypeGardenlet),
	)
})
