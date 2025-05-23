// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/validation"
)

var _ = Describe("#ValidateControllerDeployment", func() {
	var controllerDeployment *ControllerDeployment

	BeforeEach(func() {
		controllerDeployment = &ControllerDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "deployment-abc",
			},
			Helm: &HelmControllerDeployment{
				RawChart: []byte("foo"),
			},
		}
	})

	DescribeTable("ControllerRegistration metadata",
		func(objectMeta metav1.ObjectMeta, matcher gomegatypes.GomegaMatcher) {
			controllerDeployment.ObjectMeta = objectMeta

			Expect(ValidateControllerDeployment(controllerDeployment)).To(matcher)
		},

		Entry("should forbid ControllerDeployment with empty metadata",
			metav1.ObjectMeta{},
			ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			}))),
		),
		Entry("should forbid ControllerDeployment with empty name",
			metav1.ObjectMeta{Name: ""},
			ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			}))),
		),
		Entry("should forbid ControllerDeployment with '.' in the name (not a DNS-1123 label compliant name)",
			metav1.ObjectMeta{Name: "extension-abc.test"},
			ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("metadata.name"),
			}))),
		),
		Entry("should forbid ControllerDeployment with '_' in the name (not a DNS-1123 subdomain)",
			metav1.ObjectMeta{Name: "extension-abc_test"},
			ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("metadata.name"),
			}))),
		),
	)

	It("should require setting a deployment configuration", func() {
		controllerDeployment.Type = ""
		controllerDeployment.Helm = nil

		Expect(ValidateControllerDeployment(controllerDeployment)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(field.ErrorTypeForbidden),
			"Detail": Equal("must use either helm or a custom deployment configuration"),
		}))))
	})

	Context("helm type", func() {
		BeforeEach(func() {
			controllerDeployment.Helm = &HelmControllerDeployment{
				Values: &apiextensionsv1.JSON{
					Raw: []byte(`{"foo":["bar","baz"]}`),
				},
			}
		})

		It("should not allow both rawChart and OCIRepository", func() {
			controllerDeployment.Helm = &HelmControllerDeployment{
				RawChart: []byte("foo"),
				OCIRepository: &OCIRepository{
					Ref: ptr.To("foo"),
				},
			}
			Expect(ValidateControllerDeployment(controllerDeployment)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Detail": Equal("must provide either rawChart or ociRepository"),
			}))))
		})

		Context("with rawChart", func() {
			BeforeEach(func() {
				controllerDeployment.Helm.RawChart = []byte("foo")
			})

			It("should allow a valid helm deployment configuration", func() {
				Expect(ValidateControllerDeployment(controllerDeployment)).To(BeEmpty())
			})

			It("should forbid setting type", func() {
				controllerDeployment.Type = "helm"

				Expect(ValidateControllerDeployment(controllerDeployment)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("type"),
					"Detail": ContainSubstring("if a built-in deployment type (helm) is used"),
				}))))
			})

			It("should forbid setting providerConfig", func() {
				controllerDeployment.ProviderConfig = &runtime.Unknown{}

				Expect(ValidateControllerDeployment(controllerDeployment)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("providerConfig"),
					"Detail": ContainSubstring("if a built-in deployment type (helm) is used"),
				}))))
			})

			It("should require setting rawChart", func() {
				controllerDeployment.Helm.RawChart = nil

				Expect(ValidateControllerDeployment(controllerDeployment)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("helm"),
					"Detail": ContainSubstring("must provide either"),
				}))))
			})
		})

		Context("with ociRepository", func() {
			BeforeEach(func() {
				controllerDeployment.Helm.OCIRepository = &OCIRepository{
					Repository: ptr.To("foo"),
					Tag:        ptr.To("1.0.0"),
					Digest:     ptr.To("sha256:foo"),
				}
			})

			It("should allow a valid helm deployment configuration", func() {
				Expect(ValidateControllerDeployment(controllerDeployment)).To(BeEmpty())
			})

			It("should validate required oci URL", func() {
				controllerDeployment.Helm.OCIRepository.Repository = nil

				Expect(ValidateControllerDeployment(controllerDeployment)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("helm.ociRepository"),
				}))))
			})

			It("should require either tag or digest", func() {
				controllerDeployment.Helm.OCIRepository.Tag = nil
				controllerDeployment.Helm.OCIRepository.Digest = nil

				Expect(ValidateControllerDeployment(controllerDeployment)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("helm.ociRepository"),
					"Detail": ContainSubstring("must provide either"),
				}))))
			})

			It("should validate required oci URL", func() {
				controllerDeployment.Helm.OCIRepository.Digest = ptr.To("invalid")

				Expect(ValidateControllerDeployment(controllerDeployment)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("helm.ociRepository.digest"),
				}))))
			})

			It("should require setting ociRepository", func() {
				controllerDeployment.Helm.OCIRepository = nil

				Expect(ValidateControllerDeployment(controllerDeployment)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("helm"),
				}))))
			})
		})

		Context("with ociRepository.ref", func() {
			BeforeEach(func() {
				controllerDeployment.Helm.OCIRepository = &OCIRepository{
					Ref: ptr.To("foo:v1.0.0"),
				}
			})

			It("should allow a valid helm deployment configuration", func() {
				Expect(ValidateControllerDeployment(controllerDeployment)).To(BeEmpty())
			})

			It("should not allow fields other than ref", func() {
				controllerDeployment.Helm.OCIRepository.Repository = ptr.To("foo")
				controllerDeployment.Helm.OCIRepository.Digest = ptr.To("foo")
				controllerDeployment.Helm.OCIRepository.Tag = ptr.To("foo")

				Expect(ValidateControllerDeployment(controllerDeployment)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("helm.ociRepository.repository"),
						"Detail": ContainSubstring("when ref is set"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("helm.ociRepository.tag"),
						"Detail": ContainSubstring("when ref is set"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("helm.ociRepository.digest"),
						"Detail": ContainSubstring("when ref is set"),
					})),
				))
			})
		})
	})

	Context("custom type", func() {
		BeforeEach(func() {
			controllerDeployment.Helm = nil
			controllerDeployment.Type = "custom"
			controllerDeployment.ProviderConfig = &runtime.Unknown{
				ContentType: "application/json",
				Raw:         []byte(`{"foo":"bar"}`),
			}
		})

		It("should allow a valid custom deployment configuration", func() {
			Expect(ValidateControllerDeployment(controllerDeployment)).To(BeEmpty())
		})
	})
})
