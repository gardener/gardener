// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot_test

import (
	"crypto/x509"
	"crypto/x509/pkix"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apiserver/pkg/authentication/user"

	"github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity"
	. "github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity/shoot"
)

var _ = Describe("identity", func() {
	DescribeTable("#FromUserInfoInterface",
		func(u user.Info, expectedShootNamespace, expectedShootName string, expectedIsSelfHostedShootValue bool, expectedUserType gardenletidentity.UserType) {
			shootNamespace, shootName, isSelfHostedShoot, userType := FromUserInfoInterface(u)

			Expect(shootNamespace).To(Equal(expectedShootNamespace))
			Expect(shootName).To(Equal(expectedShootName))
			Expect(isSelfHostedShoot).To(Equal(expectedIsSelfHostedShootValue))
			Expect(userType).To(Equal(expectedUserType))
		},

		Entry("nil", nil, "", "", false, gardenletidentity.UserType("")),
		Entry("no user name prefix", &user.DefaultInfo{Name: "foo"}, "", "", false, gardenletidentity.UserType("")),
		Entry("user name prefix but no groups", &user.DefaultInfo{Name: "gardener.cloud:system:shoot:foo:bar"}, "", "", false, gardenletidentity.UserType("")),
		Entry("user name prefix but shoot group not present", &user.DefaultInfo{Name: "gardener.cloud:system:shoot:foo:bar", Groups: []string{"bar"}}, "", "", false, gardenletidentity.UserType("")),
		Entry("user name prefix and shoot group", &user.DefaultInfo{Name: "gardener.cloud:system:shoot:foo:bar", Groups: []string{"gardener.cloud:system:shoots"}}, "foo", "bar", true, gardenletidentity.UserTypeGardenlet),
		Entry("gardenadm usertype", &user.DefaultInfo{Name: "gardener.cloud:gardenadm:shoot:foo:bar", Groups: []string{"gardener.cloud:system:shoots"}}, "foo", "bar", true, gardenletidentity.UserTypeGardenadm),
		Entry("Extension ServiceAccount", &user.DefaultInfo{Name: "system:serviceaccount:foo:extension-bar", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:foo"}}, "", "", false, gardenletidentity.UserTypeExtension),
	)

	DescribeTable("#FromAuthenticationV1UserInfo",
		func(u authenticationv1.UserInfo, expectedShootNamespace, expectedShootName string, expectedIsSelfHostedShootValue bool, expectedUserType gardenletidentity.UserType) {
			shootNamespace, shootName, isSelfHostedShoot, userType := FromAuthenticationV1UserInfo(u)

			Expect(shootNamespace).To(Equal(expectedShootNamespace))
			Expect(shootName).To(Equal(expectedShootName))
			Expect(isSelfHostedShoot).To(Equal(expectedIsSelfHostedShootValue))
			Expect(userType).To(Equal(expectedUserType))
		},

		Entry("no user name prefix", authenticationv1.UserInfo{Username: "foo"}, "", "", false, gardenletidentity.UserType("")),
		Entry("user name prefix but no groups", authenticationv1.UserInfo{Username: "gardener.cloud:system:shoot:foo:bar"}, "", "", false, gardenletidentity.UserType("")),
		Entry("user name prefix but shoot group not present", authenticationv1.UserInfo{Username: "gardener.cloud:system:shoot:foo:bar", Groups: []string{"bar"}}, "", "", false, gardenletidentity.UserType("")),
		Entry("user name prefix and shoot group", authenticationv1.UserInfo{Username: "gardener.cloud:system:shoot:foo:bar", Groups: []string{"gardener.cloud:system:shoots"}}, "foo", "bar", true, gardenletidentity.UserTypeGardenlet),
		Entry("gardenadm usertype", authenticationv1.UserInfo{Username: "gardener.cloud:gardenadm:shoot:foo:bar", Groups: []string{"gardener.cloud:system:shoots"}}, "foo", "bar", true, gardenletidentity.UserTypeGardenadm),
		Entry("Extension ServiceAccount", authenticationv1.UserInfo{Username: "system:serviceaccount:foo:extension-bar", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:foo"}, Extra: map[string]authenticationv1.ExtraValue{}}, "", "", false, gardenletidentity.UserTypeExtension),
	)

	DescribeTable("#FromCertificateSigningRequest",
		func(csr *x509.CertificateRequest, expectedShootNamespace, expectedShootName string, expectedIsSelfHostedShootValue bool, expectedUserType gardenletidentity.UserType) {
			shootNamespace, shootName, isSelfHostedShoot, userType := FromCertificateSigningRequest(csr)

			Expect(shootNamespace).To(Equal(expectedShootNamespace))
			Expect(shootName).To(Equal(expectedShootName))
			Expect(isSelfHostedShoot).To(Equal(expectedIsSelfHostedShootValue))
			Expect(userType).To(Equal(expectedUserType))
		},

		Entry("no user name prefix", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "foo"}}, "", "", false, gardenletidentity.UserType("")),
		Entry("user name prefix but no groups", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:shoot:foo:bar"}}, "", "", false, gardenletidentity.UserType("")),
		Entry("user name prefix but shoot group not present", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:shoot:foo:bar", Organization: []string{"bar"}}}, "", "", false, gardenletidentity.UserType("")),
		Entry("user name prefix and shoot group", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:shoot:foo:bar", Organization: []string{"gardener.cloud:system:shoots"}}}, "foo", "bar", true, gardenletidentity.UserTypeGardenlet),
		Entry("gardenadm usertype", &x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:gardenadm:shoot:foo:bar", Organization: []string{"gardener.cloud:system:shoots"}}}, "foo", "bar", true, gardenletidentity.UserTypeGardenadm),
	)
})
