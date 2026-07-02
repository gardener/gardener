// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/gardener/gardener/pkg/api/core/validation"
	. "github.com/gardener/gardener/pkg/apis/core"
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
					Ref: new("foo"),
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
					Repository: new("foo"),
					Tag:        new("1.0.0"),
					Digest:     new("sha256:foo"),
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
				controllerDeployment.Helm.OCIRepository.Digest = new("invalid")

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
					Ref: new("foo:v1.0.0"),
				}
			})

			It("should allow a valid helm deployment configuration", func() {
				Expect(ValidateControllerDeployment(controllerDeployment)).To(BeEmpty())
			})

			It("should not allow fields other than ref", func() {
				controllerDeployment.Helm.OCIRepository.Repository = new("foo")
				controllerDeployment.Helm.OCIRepository.Digest = new("foo")
				controllerDeployment.Helm.OCIRepository.Tag = new("foo")

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

	Context("resources", func() {
		It("should allow Secret and ConfigMap references", func() {
			controllerDeployment.Resources = []NamedResourceReference{
				{Name: "creds", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "my-secret"}},
				{Name: "cfg", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "ConfigMap", Name: "my-config"}},
			}
			Expect(ValidateControllerDeployment(controllerDeployment)).To(BeEmpty())
		})

		It("should reject duplicate resource names", func() {
			controllerDeployment.Resources = []NamedResourceReference{
				{Name: "creds", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "my-secret"}},
				{Name: "creds", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "ConfigMap", Name: "my-config"}},
			}
			Expect(ValidateControllerDeployment(controllerDeployment)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeDuplicate),
				"Field": Equal("resources[1].name"),
			}))))
		})

		It("should reject empty name", func() {
			controllerDeployment.Resources = []NamedResourceReference{
				{Name: "", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "my-secret"}},
			}
			Expect(ValidateControllerDeployment(controllerDeployment)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("resources[0].name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("resources[0].name"),
					"Detail": ContainSubstring("alphanumeric"),
				})),
			))
		})

		It("should reject unsupported kind", func() {
			controllerDeployment.Resources = []NamedResourceReference{
				{Name: "x", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Pod", Name: "anything"}},
			}
			Expect(ValidateControllerDeployment(controllerDeployment)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("resources[0].resourceRef"),
			}))))
		})

		It("should reject unsupported apiVersion", func() {
			controllerDeployment.Resources = []NamedResourceReference{
				{Name: "x", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "apps/v1", Kind: "Secret", Name: "my-secret"}},
			}
			Expect(ValidateControllerDeployment(controllerDeployment)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("resources[0].resourceRef"),
			}))))
		})

		It("should reject WorkloadIdentity references", func() {
			controllerDeployment.Resources = []NamedResourceReference{
				{Name: "x", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "security.gardener.cloud/v1alpha1", Kind: "WorkloadIdentity", Name: "wi"}},
			}
			Expect(ValidateControllerDeployment(controllerDeployment)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("resources[0].resourceRef"),
			}))))
		})

		It("should reject non-alphanumeric resource names", func() {
			controllerDeployment.Resources = []NamedResourceReference{
				{Name: "my-creds", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "my-secret"}},
			}
			Expect(ValidateControllerDeployment(controllerDeployment)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("resources[0].name"),
				"Detail": ContainSubstring("alphanumeric"),
			}))))
		})
	})

	Context("helm values templates", func() {
		BeforeEach(func() {
			controllerDeployment.Helm = &HelmControllerDeployment{
				RawChart: []byte("foo"),
			}
			controllerDeployment.Resources = []NamedResourceReference{
				{Name: "creds", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "my-secret"}},
				{Name: "foo", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo-secret"}},
				{Name: "bar", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "ConfigMap", Name: "bar-cm"}},
			}
		})

		It("should accept a valid resource template reference", func() {
			controllerDeployment.Helm.Values = &apiextensionsv1.JSON{Raw: []byte(`{"x":"{{ .resources.creds.data.token }}"}`)}
			Expect(ValidateControllerDeployment(controllerDeployment)).To(BeEmpty())
		})

		It("should accept a valid resource template reference without surrounding spaces", func() {
			controllerDeployment.Helm.Values = &apiextensionsv1.JSON{Raw: []byte(`{"x":"{{.resources.creds.data.token}}"}`)}
			Expect(ValidateControllerDeployment(controllerDeployment)).To(BeEmpty())
		})

		It("should accept multiple valid resource template references", func() {
			controllerDeployment.Helm.Values = &apiextensionsv1.JSON{Raw: []byte(`{"a":"{{ .resources.foo.data.x }}","b":"{{ .resources.bar.data.y }}"}`)}
			Expect(ValidateControllerDeployment(controllerDeployment)).To(BeEmpty())
		})

		It("should reject a template with non-alphanumeric resource name", func() {
			controllerDeployment.Helm.Values = &apiextensionsv1.JSON{Raw: []byte(`{"x":"{{ .resources.my-creds.data.token }}"}`)}
			Expect(ValidateControllerDeployment(controllerDeployment)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("helm.values"),
			}))))
		})

		It("should reject a template with non-alphanumeric data key", func() {
			controllerDeployment.Helm.Values = &apiextensionsv1.JSON{Raw: []byte(`{"x":"{{ .resources.creds.data.my-token }}"}`)}
			Expect(ValidateControllerDeployment(controllerDeployment)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("helm.values"),
			}))))
		})

		It("should reject a template that does not reference resources", func() {
			controllerDeployment.Helm.Values = &apiextensionsv1.JSON{Raw: []byte(`{"x":"{{ .other.thing }}"}`)}
			Expect(ValidateControllerDeployment(controllerDeployment)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("helm.values"),
			}))))
		})

		It("should reject a template that uses whitespace trim markers", func() {
			controllerDeployment.Helm.Values = &apiextensionsv1.JSON{Raw: []byte(`{"x":"{{- .resources.creds.data.token -}}"}`)}
			Expect(ValidateControllerDeployment(controllerDeployment)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("helm.values"),
			}))))
		})

		It("should reject a template with extra path segments", func() {
			controllerDeployment.Helm.Values = &apiextensionsv1.JSON{Raw: []byte(`{"x":"{{ .resources.creds.data.token.extra }}"}`)}
			Expect(ValidateControllerDeployment(controllerDeployment)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("helm.values"),
			}))))
		})

		It("should report all invalid templates", func() {
			controllerDeployment.Helm.Values = &apiextensionsv1.JSON{Raw: []byte(`{"a":"{{ .resources.creds.data.token }}","b":"{{ .other }}","c":"{{ .resources.bad-name.data.k }}"}`)}
			Expect(ValidateControllerDeployment(controllerDeployment)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("helm.values"),
					"BadValue": Equal("{{ .other }}"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("helm.values"),
					"BadValue": Equal("{{ .resources.bad-name.data.k }}"),
				})),
			))
		})

		It("should reject a template referencing an undeclared resource name", func() {
			controllerDeployment.Helm.Values = &apiextensionsv1.JSON{Raw: []byte(`{"x":"{{ .resources.undeclared.data.token }}"}`)}
			Expect(ValidateControllerDeployment(controllerDeployment)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("helm.values"),
				"Detail": ContainSubstring(`template references resource "undeclared" which is not declared in the resources list`),
			}))))
		})

		It("should accept values without any template expressions", func() {
			controllerDeployment.Helm.Values = &apiextensionsv1.JSON{Raw: []byte(`{"x":"plain string","y":42}`)}
			Expect(ValidateControllerDeployment(controllerDeployment)).To(BeEmpty())
		})
	})
})
