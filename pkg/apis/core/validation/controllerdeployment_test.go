// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
				RawChart: []byte("foo"),
				Values: &apiextensionsv1.JSON{
					Raw: []byte(`{"foo":["bar","baz"]}`),
				},
			}
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
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("helm.rawChart"),
			}))))
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
