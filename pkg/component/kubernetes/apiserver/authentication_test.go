// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
)

var _ = Describe("Authentication", func() {
	Describe("#ComputeAuthenticationConfigRawConfig", func() {
		DescribeTable("should compute correct AuthenticationConfiguration",
			func(oidc *gardencorev1beta1.OIDCConfig, expectetedResult string, errorMatcher gomegatypes.GomegaMatcher) {
				res, err := ComputeAuthenticationConfigRawConfig(oidc)

				Expect(err).To(errorMatcher)
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
			}, `apiVersion: apiserver.config.k8s.io/v1beta1
jwt:
- claimMappings:
    groups:
      claim: some-groups-claim
      prefix: some-groups-prefix
    uid: {}
    username:
      claim: some-user-claim
      prefix: some-user-prefix
  claimValidationRules:
  - claim: claim1
    requiredValue: value1
  issuer:
    audiences:
    - some-client-id
    certificateAuthority: some-ca-bundle
    url: https://issuer.url.com
kind: AuthenticationConfiguration
`, BeNil()),
			Entry("should return configuration with defaulted username values", &gardencorev1beta1.OIDCConfig{
				ClientID:  ptr.To("some-client-id"),
				IssuerURL: ptr.To("https://issuer.url.com"),
			}, `apiVersion: apiserver.config.k8s.io/v1beta1
jwt:
- claimMappings:
    groups: {}
    uid: {}
    username:
      claim: sub
      prefix: https://issuer.url.com#
  issuer:
    audiences:
    - some-client-id
    url: https://issuer.url.com
kind: AuthenticationConfiguration
`, BeNil()),
			Entry("should return correct configuration when usernamePrefix is '-'", &gardencorev1beta1.OIDCConfig{
				ClientID:       ptr.To("some-client-id"),
				IssuerURL:      ptr.To("https://issuer.url.com"),
				UsernameClaim:  ptr.To("claim"),
				UsernamePrefix: ptr.To("-"),
			}, `apiVersion: apiserver.config.k8s.io/v1beta1
jwt:
- claimMappings:
    groups: {}
    uid: {}
    username:
      claim: claim
      prefix: ""
  issuer:
    audiences:
    - some-client-id
    url: https://issuer.url.com
kind: AuthenticationConfiguration
`, BeNil()),
			Entry("should return correct configuration when usernameClaim is 'email' and prefix not specified", &gardencorev1beta1.OIDCConfig{
				ClientID:       ptr.To("some-client-id"),
				IssuerURL:      ptr.To("https://issuer.url.com"),
				UsernameClaim:  ptr.To("email"),
				UsernamePrefix: ptr.To(""),
			}, `apiVersion: apiserver.config.k8s.io/v1beta1
jwt:
- claimMappings:
    groups: {}
    uid: {}
    username:
      claim: email
      prefix: ""
  issuer:
    audiences:
    - some-client-id
    url: https://issuer.url.com
kind: AuthenticationConfiguration
`, BeNil()),
			Entry("should return correct configuration when groupsClaim is set but prefix is not", &gardencorev1beta1.OIDCConfig{
				ClientID:    ptr.To("some-client-id"),
				IssuerURL:   ptr.To("https://issuer.url.com"),
				GroupsClaim: ptr.To("some-groups-claim"),
			}, `apiVersion: apiserver.config.k8s.io/v1beta1
jwt:
- claimMappings:
    groups:
      claim: some-groups-claim
      prefix: ""
    uid: {}
    username:
      claim: sub
      prefix: https://issuer.url.com#
  issuer:
    audiences:
    - some-client-id
    url: https://issuer.url.com
kind: AuthenticationConfiguration
`, BeNil()),
		)
	})

})
