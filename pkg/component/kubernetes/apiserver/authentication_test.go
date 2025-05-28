// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiserverv1beta1 "k8s.io/apiserver/pkg/apis/apiserver/v1beta1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
)

var _ = Describe("Authentication", func() {
	Describe("#ConfigureJWTAuthenticators", func() {
		DescribeTable("should compute correct slice of JWTAuthenticators",
			func(oidc *gardencorev1beta1.OIDCConfig, expectetedResult []apiserverv1beta1.JWTAuthenticator) {
				res := ConfigureJWTAuthenticators(oidc)

				Expect(res).To(Equal(expectetedResult))
			},
			Entry("should return correct configuration when all values are present", &gardencorev1beta1.OIDCConfig{
				CABundle:     ptr.To("some-ca-bundle"),
				ClientID:     ptr.To("some-client-id"),
				GroupsClaim:  ptr.To("some-groups-claim"),
				GroupsPrefix: ptr.To("some-groups-prefix"),
				IssuerURL:    ptr.To("https://issuer.url.com"),
				RequiredClaims: map[string]string{
					"claim1": "value1",
				},
				UsernameClaim:  ptr.To("some-user-claim"),
				UsernamePrefix: ptr.To("some-user-prefix"),
			},
				[]apiserverv1beta1.JWTAuthenticator{
					{
						ClaimMappings: apiserverv1beta1.ClaimMappings{
							Groups: apiserverv1beta1.PrefixedClaimOrExpression{
								Claim:  "some-groups-claim",
								Prefix: ptr.To("some-groups-prefix"),
							},
							Username: apiserverv1beta1.PrefixedClaimOrExpression{
								Claim:  "some-user-claim",
								Prefix: ptr.To("some-user-prefix"),
							},
						},
						ClaimValidationRules: []apiserverv1beta1.ClaimValidationRule{
							{
								Claim:         "claim1",
								RequiredValue: "value1",
							},
						},
						Issuer: apiserverv1beta1.Issuer{
							Audiences:            []string{"some-client-id"},
							CertificateAuthority: "some-ca-bundle",
							URL:                  "https://issuer.url.com",
						},
					},
				}),
			Entry("should return configuration with defaulted username values", &gardencorev1beta1.OIDCConfig{
				ClientID:  ptr.To("some-client-id"),
				IssuerURL: ptr.To("https://issuer.url.com"),
			},
				[]apiserverv1beta1.JWTAuthenticator{
					{
						ClaimMappings: apiserverv1beta1.ClaimMappings{
							Username: apiserverv1beta1.PrefixedClaimOrExpression{
								Claim:  "sub",
								Prefix: ptr.To("https://issuer.url.com#"),
							},
						},
						Issuer: apiserverv1beta1.Issuer{
							Audiences: []string{"some-client-id"},
							URL:       "https://issuer.url.com",
						},
					},
				}),
			Entry("should return correct configuration when usernamePrefix is '-'", &gardencorev1beta1.OIDCConfig{
				ClientID:       ptr.To("some-client-id"),
				IssuerURL:      ptr.To("https://issuer.url.com"),
				UsernameClaim:  ptr.To("claim"),
				UsernamePrefix: ptr.To("-"),
			},
				[]apiserverv1beta1.JWTAuthenticator{
					{
						ClaimMappings: apiserverv1beta1.ClaimMappings{
							Username: apiserverv1beta1.PrefixedClaimOrExpression{
								Claim:  "claim",
								Prefix: ptr.To(""),
							},
						},
						Issuer: apiserverv1beta1.Issuer{
							Audiences: []string{"some-client-id"},
							URL:       "https://issuer.url.com",
						},
					},
				}),
			Entry("should return correct configuration when usernameClaim is 'email' and prefix not specified", &gardencorev1beta1.OIDCConfig{
				ClientID:       ptr.To("some-client-id"),
				IssuerURL:      ptr.To("https://issuer.url.com"),
				UsernameClaim:  ptr.To("email"),
				UsernamePrefix: ptr.To(""),
			},
				[]apiserverv1beta1.JWTAuthenticator{
					{
						ClaimMappings: apiserverv1beta1.ClaimMappings{
							Username: apiserverv1beta1.PrefixedClaimOrExpression{
								Claim:  "email",
								Prefix: ptr.To(""),
							},
						},
						Issuer: apiserverv1beta1.Issuer{
							Audiences: []string{"some-client-id"},
							URL:       "https://issuer.url.com",
						},
					},
				}),
			Entry("should return correct configuration when groupsClaim is set but prefix is not", &gardencorev1beta1.OIDCConfig{
				ClientID:    ptr.To("some-client-id"),
				IssuerURL:   ptr.To("https://issuer.url.com"),
				GroupsClaim: ptr.To("some-groups-claim"),
			},
				[]apiserverv1beta1.JWTAuthenticator{
					{
						ClaimMappings: apiserverv1beta1.ClaimMappings{
							Groups: apiserverv1beta1.PrefixedClaimOrExpression{
								Claim:  "some-groups-claim",
								Prefix: ptr.To(""),
							},
							Username: apiserverv1beta1.PrefixedClaimOrExpression{
								Claim:  "sub",
								Prefix: ptr.To("https://issuer.url.com#"),
							},
						},
						Issuer: apiserverv1beta1.Issuer{
							Audiences: []string{"some-client-id"},
							URL:       "https://issuer.url.com",
						},
					},
				}),
		)
	})
})
