// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/settings"
)

const empty = ""

// spellchecker:off
const validCert = `
-----BEGIN CERTIFICATE-----
MIICljCCAX4CCQDWZEelmpcGpTANBgkqhkiG9w0BAQsFADANMQswCQYDVQQGEwJC
RzAeFw0xOTA4MjcxNTAyNDFaFw0xOTA5MjYxNTAyNDFaMA0xCzAJBgNVBAYTAkJH
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA6eXDMFqcsMSfysNC7RAL
pYcOGYtA2jklZWKMx7lBRmYV/7/FPStjItbRV6HqieDJOj6f8eE6+0g9hVxASv8q
KoGPxdG4OFQNACCtDGRFfZn6o4qG1w+JMuQX9FZ3+b2ZZg6Chb/CTjiTHaMhhNTO
KOHTvCPIVKjIxxGKSUoS5wGGrG7Zjxaoepc8LnsK/PWQpGB9Oka9tOuJ5skb8Qqr
h5VDXhJqjy1GVXDt+BhadcPEp7XluOSkKUtUFq0c0gdueXOgjBkea2BmfyzmBo1j
6IMoX+0fEVVO73Hhu/zfEY13QWUHonXdXQuwmDUUy4s0YJZpTTY/HI0dMdrgz3Cm
GQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQAX6KNkRFJFFU6I2S/0GFbxhr3Mno1r
LpCq0/MowXGjMjDlEQPKcOqoXdHFyPzDQqjF03NlzrkacYfy/huExFt6b3jpQTSh
GVa3mYEA7P3aFXjVSbVhKLTmHrY9nDWqGCNfEXg2cs+qNiXvn4d6OL854SAfgNte
gYHP5ew4l3NZVa5ieX94fHE0UQ0ApDa5cM6KWKj8Z4Qd4kzXwcVOwsPQ9GCrfiXC
zntZlrSTQGEBwAk3OsDJe9dOBsgR7IWiat5leJQ60oQ9xpSj9JalrMBZnNKpO1RI
664+oMpUFHeSgdQ4kA90lir7X6G6oAsfaFLC7uPUGexAGbHzX1FxjbFA
-----END CERTIFICATE-----
`

// spellchecker:on

// type providerFunc func() field.ErrorList
type provider interface {
	providerFunc() field.ErrorList
	preset() settings.Preset
}

func TestValidation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "APIs Settings Validation Suite")
}

func validationAssertions(p provider) {
	var presetSpec *settings.OpenIDConnectPresetSpec

	BeforeEach(func() {
		presetSpec = p.preset().GetPresetSpec()
	})

	It("should allow valid resource", func() {
		errorList := p.providerFunc()
		Expect(errorList).To(BeEmpty())
	})

	Context("shootSelector", func() {
		It("invalid selector", func() {
			p.preset().GetPresetSpec().ShootSelector = &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "foo",
						Operator: metav1.LabelSelectorOpExists,
						Values:   []string{"bar"},
					}},
			}

			errorList := p.providerFunc()

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec.shootSelector.matchExpressions[0].values"),
				"Detail": Equal("may not be specified when `operator` is 'Exists' or 'DoesNotExist'"),
			})),
			))
		})
	})

	Context("server", func() {
		DescribeTable("issuerURL",
			func(value, detail string, errorType field.ErrorType) {
				p.preset().GetPresetSpec().Server.IssuerURL = value
				errorList := p.providerFunc()

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(errorType),
					"Field":    Equal("spec.server.issuerURL"),
					"BadValue": Equal(value),
					"Detail":   Equal(detail),
				})),
				))
			},
			Entry("with empty value", empty, "must not be empty", field.ErrorTypeRequired),
			Entry("with no host", "https://", "must be a valid URL", field.ErrorTypeInvalid),
			Entry("with invalid scheme", "http://foo", "must have https scheme", field.ErrorTypeInvalid),
		)
		Context("caBundle", func() {
			It("valid bundle", func() {
				cert := validCert
				p.preset().GetPresetSpec().Server.CABundle = &cert

				errorList := p.providerFunc()
				Expect(errorList).To(BeEmpty())
			})
			It("invalid bundle", func() {
				brokenCert := "-----BEGIN CERTIFICATE-----"
				p.preset().GetPresetSpec().Server.CABundle = &brokenCert

				errorList := p.providerFunc()

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("spec.server.caBundle"),
					"BadValue": Equal(brokenCert),
					"Detail":   Equal("must be a valid PEM-encoded certificate"),
				})),
				))
			})
		})
		Context("groupsClaim", func() {
			It("valid claims", func() {
				claim := "foo-groups"
				p.preset().GetPresetSpec().Server.GroupsClaim = &claim

				errorList := p.providerFunc()

				Expect(errorList).To(BeEmpty())
			})
			It("empty claim", func() {
				claim := empty
				p.preset().GetPresetSpec().Server.GroupsClaim = &claim

				errorList := p.providerFunc()

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("spec.server.groupsClaim"),
					"BadValue": Equal(empty),
					"Detail":   Equal("must not be empty"),
				})),
				))
			})
		})
		Context("groupsPrefix", func() {
			It("valid prefix", func() {
				prefix := "foo-groups"
				p.preset().GetPresetSpec().Server.GroupsPrefix = &prefix

				errorList := p.providerFunc()

				Expect(errorList).To(BeEmpty())
			})
			It("empty prefix", func() {
				prefix := empty
				p.preset().GetPresetSpec().Server.GroupsPrefix = &prefix

				errorList := p.providerFunc()

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("spec.server.groupsPrefix"),
					"BadValue": Equal(empty),
					"Detail":   Equal("must not be empty"),
				})),
				))
			})
		})
		Context("signingAlgs", func() {
			DescribeTable("valid algs", func(alg string) {
				p.preset().GetPresetSpec().Server.SigningAlgs = []string{alg}
				errorList := p.providerFunc()

				Expect(errorList).To(BeEmpty())
			},
				Entry("RS256", "RS256"),
				Entry("RS384", "RS384"),
				Entry("RS512", "RS512"),
				Entry("ES256", "ES256"),
				Entry("ES384", "ES384"),
				Entry("ES512", "ES512"),
				Entry("PS256", "PS256"),
				Entry("PS384", "PS384"),
				Entry("PS512", "PS512"),
			)

			It("invalid algs", func() {
				p.preset().GetPresetSpec().Server.SigningAlgs = []string{"foo", "bar"}

				errorList := p.providerFunc()

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("spec.server.signingAlgs[0]"),
					"BadValue": Equal("foo"),
					"Detail":   Equal("must be one of: [ES256 ES384 ES512 PS256 PS384 PS512 RS256 RS384 RS512]"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("spec.server.signingAlgs[1]"),
					"BadValue": Equal("bar"),
					"Detail":   Equal("must be one of: [ES256 ES384 ES512 PS256 PS384 PS512 RS256 RS384 RS512]"),
				})),
				))
			})
			It("duplicate algs", func() {
				p.preset().GetPresetSpec().Server.SigningAlgs = []string{"ES256", "ES384", "ES256"}

				errorList := p.providerFunc()

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeDuplicate),
					"Field":    Equal("spec.server.signingAlgs[2]"),
					"BadValue": Equal("ES256"),
				})),
				))
			})
			It("empty algs", func() {
				p.preset().GetPresetSpec().Server.SigningAlgs = []string{}

				errorList := p.providerFunc()

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.server.signingAlgs"),
					"Detail": Equal("must not be empty"),
				})),
				))
			})
		})
		Context("usernameClaim", func() {
			It("valid claim", func() {
				claim := "foo-claims"
				p.preset().GetPresetSpec().Server.UsernameClaim = &claim

				errorList := p.providerFunc()

				Expect(errorList).To(BeEmpty())
			})
			It("empty claim", func() {
				claim := empty
				p.preset().GetPresetSpec().Server.UsernameClaim = &claim

				errorList := p.providerFunc()

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("spec.server.usernameClaim"),
					"BadValue": Equal(empty),
					"Detail":   Equal("must not be empty"),
				})),
				))
			})
		})
		Context("usernamePrefix", func() {
			It("valid prefix", func() {
				prefix := "foo-prefixs"
				p.preset().GetPresetSpec().Server.UsernamePrefix = &prefix

				errorList := p.providerFunc()

				Expect(errorList).To(BeEmpty())
			})
			It("empty prefix", func() {
				prefix := empty
				p.preset().GetPresetSpec().Server.UsernamePrefix = &prefix

				errorList := p.providerFunc()

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("spec.server.usernamePrefix"),
					"BadValue": Equal(empty),
					"Detail":   Equal("must not be empty"),
				})),
				))
			})
		})
	})

	Context("client", func() {
		BeforeEach(func() {
			presetSpec.Client = &settings.OpenIDConnectClientAuthentication{}
		})

		Context("secret", func() {
			It("valid secret", func() {
				secret := "some secret"
				presetSpec.Client.Secret = &secret

				errorList := p.providerFunc()

				Expect(errorList).To(BeEmpty())
			})
			It("empty secret", func() {
				secret := empty
				presetSpec.Client.Secret = &secret

				errorList := p.providerFunc()

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("spec.client.secret"),
					"BadValue": Equal(empty),
					"Detail":   Equal("must not be empty"),
				})),
				))
			})
		})

		Context("extraConfigs", func() {
			DescribeTable("fobideen config", func(key string) {
				p.preset().GetPresetSpec().Client.ExtraConfig = map[string]string{key: "some-key"}
				errorList := p.providerFunc()

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal(fmt.Sprintf("spec.client.extraConfig[%s]", key)),
					"Detail": Equal("cannot be any of [client-id client-secret id-token idp-certificate-authority idp-certificate-authority-data idp-issuer-url refresh-token]"),
				}))))
			},
				Entry("idp-issuer-url", "idp-issuer-url"),
				Entry("client-id", "client-id"),
				Entry("client-secret", "client-secret"),
				Entry("idp-certificate-authority", "idp-certificate-authority"),
				Entry("idp-certificate-authority-data", "idp-certificate-authority-data"),
				Entry("id-token", "id-token"),
				Entry("refresh-token", "refresh-token"),
			)

			It("empty config key", func() {
				presetSpec.Client.ExtraConfig = map[string]string{"foo": ""}

				errorList := p.providerFunc()

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.client.extraConfig[foo]"),
					"Detail": Equal("must not be empty"),
				})),
				))
			})
		})
	})
}
