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

	. "github.com/gardener/gardener/pkg/admissioncontroller/gardenletidentity/shoot"
)

var _ = Describe("identity", func() {
	Describe("#FromUserInfoInterface", func() {
		test := func(u user.Info, expectedShootNamespace, expectedShootName string, expectedIsAutonomousShootValue bool, expectedUserType UserType) {
			shootNamespace, shootName, isAutonomousShoot, userType := FromUserInfoInterface(u)

			Expect(shootNamespace).To(Equal(expectedShootNamespace))
			Expect(shootName).To(Equal(expectedShootName))
			Expect(isAutonomousShoot).To(Equal(expectedIsAutonomousShootValue))
			Expect(userType).To(Equal(expectedUserType))
		}

		It("nil", func() {
			test(nil, "", "", false, "")
		})

		It("no user name prefix", func() {
			test(&user.DefaultInfo{Name: "foo"}, "", "", false, "")
		})

		It("user name prefix but no groups", func() {
			test(&user.DefaultInfo{Name: "gardener.cloud:system:shoot:foo:bar"}, "", "", false, "")
		})

		It("user name prefix but shoot group not present", func() {
			test(&user.DefaultInfo{Name: "gardener.cloud:system:shoot:foo:bar", Groups: []string{"bar"}}, "", "", false, "")
		})

		It("user name prefix and shoot group", func() {
			test(&user.DefaultInfo{Name: "gardener.cloud:system:shoot:foo:bar", Groups: []string{"gardener.cloud:system:shoots"}}, "foo", "bar", true, UserTypeGardenlet)
		})

		It("Extension ServiceAccount", func() {
			test(&user.DefaultInfo{Name: "system:serviceaccount:foo:extension-bar", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:foo"}}, "", "", false, UserTypeExtension)
		})
	})

	Describe("#FromAuthenticationV1UserInfo", func() {
		test := func(u authenticationv1.UserInfo, expectedShootNamespace, expectedShootName string, expectedIsAutonomousShootValue bool, expectedUserType UserType) {
			shootNamespace, shootName, isAutonomousShoot, userType := FromAuthenticationV1UserInfo(u)

			Expect(shootNamespace).To(Equal(expectedShootNamespace))
			Expect(shootName).To(Equal(expectedShootName))
			Expect(isAutonomousShoot).To(Equal(expectedIsAutonomousShootValue))
			Expect(userType).To(Equal(expectedUserType))
		}

		It("no user name prefix", func() {
			test(authenticationv1.UserInfo{Username: "foo"}, "", "", false, "")
		})

		It("user name prefix but no groups", func() {
			test(authenticationv1.UserInfo{Username: "gardener.cloud:system:shoot:foo:bar"}, "", "", false, "")
		})

		It("user name prefix but shoot group not present", func() {
			test(authenticationv1.UserInfo{Username: "gardener.cloud:system:shoot:foo:bar", Groups: []string{"bar"}}, "", "", false, "")
		})

		It("user name prefix and shoot group", func() {
			test(authenticationv1.UserInfo{Username: "gardener.cloud:system:shoot:foo:bar", Groups: []string{"gardener.cloud:system:shoots"}}, "foo", "bar", true, UserTypeGardenlet)
		})

		It("Extension ServiceAccount", func() {
			test(authenticationv1.UserInfo{Username: "system:serviceaccount:foo:extension-bar", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:foo"}, Extra: map[string]authenticationv1.ExtraValue{}}, "", "", false, UserTypeExtension)
		})
	})

	Describe("#FromCertificateSigningRequest", func() {
		test := func(csr *x509.CertificateRequest, expectedShootNamespace, expectedShootName string, expectedIsAutonomousShootValue bool, expectedUserType UserType) {
			shootNamespace, shootName, isAutonomousShoot, userType := FromCertificateSigningRequest(csr)

			Expect(shootNamespace).To(Equal(expectedShootNamespace))
			Expect(shootName).To(Equal(expectedShootName))
			Expect(isAutonomousShoot).To(Equal(expectedIsAutonomousShootValue))
			Expect(userType).To(Equal(expectedUserType))
		}

		It("no user name prefix", func() {
			test(&x509.CertificateRequest{Subject: pkix.Name{CommonName: "foo"}}, "", "", false, "")
		})

		It("user name prefix but no groups", func() {
			test(&x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:shoot:foo:bar"}}, "", "", false, "")
		})

		It("user name prefix but shoot group not present", func() {
			test(&x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:shoot:foo:bar", Organization: []string{"bar"}}}, "", "", false, "")
		})

		It("user name prefix and shoot group", func() {
			test(&x509.CertificateRequest{Subject: pkix.Name{CommonName: "gardener.cloud:system:shoot:foo:bar", Organization: []string{"gardener.cloud:system:shoots"}}}, "foo", "bar", true, UserTypeGardenlet)
		})
	})
})
