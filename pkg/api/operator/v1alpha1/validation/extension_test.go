// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

var _ = Describe("Validation Tests", func() {
	var extension *operatorv1alpha1.Extension

	BeforeEach(func() {
		extension = &operatorv1alpha1.Extension{
			Spec: operatorv1alpha1.ExtensionSpec{
				Deployment: &operatorv1alpha1.Deployment{
					ExtensionDeployment: &operatorv1alpha1.ExtensionDeploymentSpec{
						DeploymentSpec: operatorv1alpha1.DeploymentSpec{
							Helm: &operatorv1alpha1.ExtensionHelm{
								OCIRepository: &gardencorev1.OCIRepository{
									Ref: new("example.com/chart:v1.0.0"),
								},
							},
						},
					},
				},
				Resources: []gardencorev1beta1.ControllerResource{
					{Kind: "Extension", Type: "type-a"},
				},
			},
		}
	})

	Describe("#ValidateExtension", func() {
		validateExtensionTests(
			func() field.ErrorList {
				return ValidateExtension(extension)
			},
			func() *operatorv1alpha1.Extension {
				return extension
			},
		)
	})

	Describe("#ValidateExtensionUpdate", func() {
		// Check basic extension validation is executed during update as well
		validateExtensionTests(
			func() field.ErrorList {
				return ValidateExtensionUpdate(extension, extension)
			},
			func() *operatorv1alpha1.Extension {
				return extension
			},
		)

		Context("Extension Deployment", func() {
			It("should return an error when extension deployment helm has invalid OCI repository", func() {
				extension.Spec.Deployment.ExtensionDeployment = &operatorv1alpha1.ExtensionDeploymentSpec{
					DeploymentSpec: operatorv1alpha1.DeploymentSpec{
						Helm: &operatorv1alpha1.ExtensionHelm{
							OCIRepository: &gardencorev1.OCIRepository{},
						},
					},
				}

				Expect(ValidateExtensionUpdate(extension, extension)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.deployment.extension.helm.ociRepository"),
				}))))
			})
		})

		Context("Resources", func() {
			test := func(oldPrimary, newPrimary *bool, matcher gomegatypes.GomegaMatcher) {
				GinkgoHelper()

				newExtension := extension.DeepCopy()
				extension.Spec.Resources[0].Primary = oldPrimary
				newExtension.Spec.Resources[0].Primary = newPrimary

				Expect(ValidateExtensionUpdate(extension, newExtension)).To(matcher)
			}

			BeforeEach(func() {
				extension.Spec = operatorv1alpha1.ExtensionSpec{
					Deployment: &operatorv1alpha1.Deployment{
						ExtensionDeployment: &operatorv1alpha1.ExtensionDeploymentSpec{
							DeploymentSpec: operatorv1alpha1.DeploymentSpec{
								Helm: &operatorv1alpha1.ExtensionHelm{
									OCIRepository: &gardencorev1.OCIRepository{
										Ref: new("example.com/chart:v1.0.0"),
									},
								},
							},
						},
					},
					Resources: []gardencorev1beta1.ControllerResource{
						{Kind: "Extension", Type: "type-a"},
					},
				}
			})

			It("should return no errors when field is unchanged", func() {
				test(new(true), new(true), BeEmpty())
			})

			It("should return no errors when field is set from nil to true", func() {
				test(nil, new(true), BeEmpty())
			})

			It("should return an error because the primary field is changed to false", func() {
				test(new(true), new(false), ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.resources[0].primary"),
				}))))
			})

			It("should return an error because the primary field is changed to nil", func() {
				test(new(false), nil, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.resources[0].primary"),
				}))))
			})

			It("should return an error because the primary field is changed to true", func() {
				test(new(false), new(true), ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.resources[0].primary"),
				}))))
			})
		})
	})
})

func validateExtensionTests(test func() field.ErrorList, extension func() *operatorv1alpha1.Extension) {
	It("should return no errors for basic extension", func() {
		Expect(test()).To(BeEmpty())
	})

	It("should return an error when deployment is nil", func() {
		extension().Spec.Deployment = nil

		Expect(test()).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":  Equal(field.ErrorTypeRequired),
			"Field": Equal("spec.deployment"),
		}))))
	})

	It("should return an error when neither extension nor admission deployment is specified", func() {
		extension().Spec.Deployment.ExtensionDeployment = nil
		extension().Spec.Deployment.AdmissionDeployment = nil

		Expect(test()).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":  Equal(field.ErrorTypeRequired),
			"Field": Equal("spec.deployment"),
		}))))
	})

	Context("Extension Deployment", func() {
		It("should return no errors when extension deployment is nil", func() {
			extension().Spec.Deployment.ExtensionDeployment = nil
			extension().Spec.Deployment.AdmissionDeployment = &operatorv1alpha1.AdmissionDeploymentSpec{
				RuntimeCluster: &operatorv1alpha1.DeploymentSpec{
					Helm: &operatorv1alpha1.ExtensionHelm{
						OCIRepository: &gardencorev1.OCIRepository{
							Ref: new("example.com/admission:v1.0.0"),
						},
					},
				},
			}

			Expect(test()).To(BeEmpty())
		})

		It("should return an error when extension deployment has no helm config", func() {
			extension().Spec.Deployment.ExtensionDeployment = &operatorv1alpha1.ExtensionDeploymentSpec{}

			Expect(test()).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.deployment.extension.helm"),
			}))))
		})

		It("should return an error when admission runtime deployment has no helm config", func() {
			extension().Spec.Deployment.AdmissionDeployment = &operatorv1alpha1.AdmissionDeploymentSpec{
				RuntimeCluster: &operatorv1alpha1.DeploymentSpec{},
			}

			Expect(test()).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.deployment.admission.runtimeCluster.helm"),
			}))))
		})

		It("should return an error when admission virtual deployment has no helm config", func() {
			extension().Spec.Deployment.AdmissionDeployment = &operatorv1alpha1.AdmissionDeploymentSpec{
				VirtualCluster: &operatorv1alpha1.DeploymentSpec{},
			}

			Expect(test()).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.deployment.admission.virtualCluster.helm"),
			}))))
		})

		It("should return an error when extension deployment helm has invalid OCI repository", func() {
			extension().Spec.Deployment.ExtensionDeployment = &operatorv1alpha1.ExtensionDeploymentSpec{
				DeploymentSpec: operatorv1alpha1.DeploymentSpec{
					Helm: &operatorv1alpha1.ExtensionHelm{
						OCIRepository: &gardencorev1.OCIRepository{},
					},
				},
			}

			Expect(test()).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.deployment.extension.helm.ociRepository"),
			}))))
		})

		It("should return no errors when extension deployment helm has valid OCI repository with ref", func() {
			extension().Spec.Deployment.ExtensionDeployment = &operatorv1alpha1.ExtensionDeploymentSpec{
				DeploymentSpec: operatorv1alpha1.DeploymentSpec{
					Helm: &operatorv1alpha1.ExtensionHelm{
						OCIRepository: &gardencorev1.OCIRepository{
							Ref: new("example.com/chart:v1.0.0"),
						},
					},
				},
			}

			Expect(test()).To(BeEmpty())
		})

		It("should return no errors when extension deployment helm has valid OCI repository with repository and tag", func() {
			extension().Spec.Deployment.ExtensionDeployment = &operatorv1alpha1.ExtensionDeploymentSpec{
				DeploymentSpec: operatorv1alpha1.DeploymentSpec{
					Helm: &operatorv1alpha1.ExtensionHelm{
						OCIRepository: &gardencorev1.OCIRepository{
							Repository: new("example.com/chart"),
							Tag:        new("v1.0.0"),
						},
					},
				},
			}

			Expect(test()).To(BeEmpty())
		})
	})

	Context("Admission Deployment", func() {
		It("should return no errors when admission deployment is nil", func() {
			extension().Spec.Deployment.AdmissionDeployment = nil

			Expect(test()).To(BeEmpty())
		})

		It("should return an error when runtime or virtual cluster deployment is nil", func() {
			extension().Spec.Deployment.AdmissionDeployment = &operatorv1alpha1.AdmissionDeploymentSpec{}

			Expect(test()).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.deployment.admission"),
			}))))
		})

		It("should return an error when admission runtime cluster has invalid OCI repository", func() {
			extension().Spec.Deployment.AdmissionDeployment = &operatorv1alpha1.AdmissionDeploymentSpec{
				RuntimeCluster: &operatorv1alpha1.DeploymentSpec{
					Helm: &operatorv1alpha1.ExtensionHelm{
						OCIRepository: &gardencorev1.OCIRepository{},
					},
				},
			}

			Expect(test()).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.deployment.admission.runtimeCluster.helm.ociRepository"),
			}))))
		})

		It("should return no errors when admission runtime cluster has valid OCI repository", func() {
			extension().Spec.Deployment.AdmissionDeployment = &operatorv1alpha1.AdmissionDeploymentSpec{
				RuntimeCluster: &operatorv1alpha1.DeploymentSpec{
					Helm: &operatorv1alpha1.ExtensionHelm{
						OCIRepository: &gardencorev1.OCIRepository{
							Ref: new("example.com/admission:v1.0.0"),
						},
					},
				},
			}

			Expect(test()).To(BeEmpty())
		})

		It("should return an error when admission virtual cluster has invalid OCI repository", func() {
			extension().Spec.Deployment.AdmissionDeployment = &operatorv1alpha1.AdmissionDeploymentSpec{
				VirtualCluster: &operatorv1alpha1.DeploymentSpec{
					Helm: &operatorv1alpha1.ExtensionHelm{
						OCIRepository: &gardencorev1.OCIRepository{},
					},
				},
			}

			Expect(test()).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.deployment.admission.virtualCluster.helm.ociRepository"),
			}))))
		})

		It("should return no errors when admission virtual cluster has valid OCI repository", func() {
			extension().Spec.Deployment.AdmissionDeployment = &operatorv1alpha1.AdmissionDeploymentSpec{
				VirtualCluster: &operatorv1alpha1.DeploymentSpec{
					Helm: &operatorv1alpha1.ExtensionHelm{
						OCIRepository: &gardencorev1.OCIRepository{
							Repository: new("example.com/admission"),
							Digest:     new("sha256:abc123"),
						},
					},
				},
			}

			Expect(test()).To(BeEmpty())
		})

		It("should return errors when both runtime and virtual clusters have invalid OCI repositories", func() {
			extension().Spec.Deployment.AdmissionDeployment = &operatorv1alpha1.AdmissionDeploymentSpec{
				RuntimeCluster: &operatorv1alpha1.DeploymentSpec{
					Helm: &operatorv1alpha1.ExtensionHelm{
						OCIRepository: &gardencorev1.OCIRepository{},
					},
				},
				VirtualCluster: &operatorv1alpha1.DeploymentSpec{
					Helm: &operatorv1alpha1.ExtensionHelm{
						OCIRepository: &gardencorev1.OCIRepository{},
					},
				},
			}

			errs := test()
			Expect(errs).To(HaveLen(2))
			Expect(errs).To(ContainElements(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.deployment.admission.runtimeCluster.helm.ociRepository"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.deployment.admission.virtualCluster.helm.ociRepository"),
				})),
			))
		})
	})

	Context("Resources", func() {
		It("should allow Secret and ConfigMap references", func() {
			extension().Spec.Deployment.Resources = []gardencorev1.NamedResourceReference{
				{Name: "creds", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "my-secret"}},
				{Name: "cfg", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "ConfigMap", Name: "my-config"}},
			}
			Expect(test()).To(BeEmpty())
		})

		It("should reject duplicate names", func() {
			extension().Spec.Deployment.Resources = []gardencorev1.NamedResourceReference{
				{Name: "creds", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "my-secret"}},
				{Name: "creds", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "ConfigMap", Name: "my-config"}},
			}
			Expect(test()).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeDuplicate),
				"Field": Equal("spec.deployment.resources[1].name"),
			}))))
		})

		It("should reject unsupported kind", func() {
			extension().Spec.Deployment.Resources = []gardencorev1.NamedResourceReference{
				{Name: "x", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Pod", Name: "anything"}},
			}
			Expect(test()).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("spec.deployment.resources[0].resourceRef"),
			}))))
		})

		It("should reject non-alphanumeric resource names", func() {
			extension().Spec.Deployment.Resources = []gardencorev1.NamedResourceReference{
				{Name: "my-creds", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "my-secret"}},
			}
			Expect(test()).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("spec.deployment.resources[0].name"),
				"Detail": ContainSubstring("alphanumeric"),
			}))))
		})

		It("should reject unsupported apiVersion", func() {
			extension().Spec.Deployment.Resources = []gardencorev1.NamedResourceReference{
				{Name: "x", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "apps/v1", Kind: "Secret", Name: "my-secret"}},
			}
			Expect(test()).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("spec.deployment.resources[0].resourceRef"),
			}))))
		})
	})

	Context("Values templates", func() {
		BeforeEach(func() {
			extension().Spec.Deployment.Resources = []gardencorev1.NamedResourceReference{
				{Name: "creds", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "creds-secret"}},
				{Name: "foo", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo-secret"}},
				{Name: "bar", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "ConfigMap", Name: "bar-cm"}},
			}
		})

		It("should accept a valid resource template reference in extension values", func() {
			extension().Spec.Deployment.ExtensionDeployment.Values = &apiextensionsv1.JSON{Raw: []byte(`{"x":"{{ .resources.creds.data.token }}"}`)}
			Expect(test()).To(BeEmpty())
		})

		It("should accept a valid resource template reference without surrounding spaces", func() {
			extension().Spec.Deployment.ExtensionDeployment.Values = &apiextensionsv1.JSON{Raw: []byte(`{"x":"{{.resources.creds.data.token}}"}`)}
			Expect(test()).To(BeEmpty())
		})

		It("should accept multiple valid resource template references", func() {
			extension().Spec.Deployment.ExtensionDeployment.RuntimeClusterValues = &apiextensionsv1.JSON{Raw: []byte(`{"a":"{{ .resources.foo.data.x }}","b":"{{ .resources.bar.data.y }}"}`)}
			Expect(test()).To(BeEmpty())
		})

		It("should accept a valid resource template reference in admission values", func() {
			extension().Spec.Deployment.AdmissionDeployment = &operatorv1alpha1.AdmissionDeploymentSpec{
				RuntimeCluster: &operatorv1alpha1.DeploymentSpec{
					Helm: &operatorv1alpha1.ExtensionHelm{
						OCIRepository: &gardencorev1.OCIRepository{Ref: new("example.com/admission:v1.0.0")},
					},
				},
				Values: &apiextensionsv1.JSON{Raw: []byte(`{"x":"{{ .resources.creds.data.token }}"}`)},
			}
			Expect(test()).To(BeEmpty())
		})

		It("should reject a template with non-alphanumeric resource name", func() {
			extension().Spec.Deployment.ExtensionDeployment.Values = &apiextensionsv1.JSON{Raw: []byte(`{"x":"{{ .resources.my-creds.data.token }}"}`)}
			Expect(test()).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.deployment.extension.values"),
			}))))
		})

		It("should reject a template with non-alphanumeric data key", func() {
			extension().Spec.Deployment.ExtensionDeployment.RuntimeClusterValues = &apiextensionsv1.JSON{Raw: []byte(`{"x":"{{ .resources.creds.data.my-token }}"}`)}
			Expect(test()).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.deployment.extension.runtimeClusterValues"),
			}))))
		})

		It("should reject a template that does not reference resources", func() {
			extension().Spec.Deployment.ExtensionDeployment.Values = &apiextensionsv1.JSON{Raw: []byte(`{"x":"{{ .other.thing }}"}`)}
			Expect(test()).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.deployment.extension.values"),
			}))))
		})

		It("should reject a template that uses whitespace trim markers", func() {
			extension().Spec.Deployment.ExtensionDeployment.Values = &apiextensionsv1.JSON{Raw: []byte(`{"x":"{{- .resources.creds.data.token -}}"}`)}
			Expect(test()).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.deployment.extension.values"),
			}))))
		})

		It("should reject a template with extra path segments", func() {
			extension().Spec.Deployment.ExtensionDeployment.Values = &apiextensionsv1.JSON{Raw: []byte(`{"x":"{{ .resources.creds.data.token.extra }}"}`)}
			Expect(test()).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.deployment.extension.values"),
			}))))
		})

		It("should report all invalid templates", func() {
			extension().Spec.Deployment.ExtensionDeployment.Values = &apiextensionsv1.JSON{Raw: []byte(`{"a":"{{ .resources.creds.data.token }}","b":"{{ .other }}","c":"{{ .resources.bad-name.data.k }}"}`)}
			Expect(test()).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("spec.deployment.extension.values"),
					"BadValue": Equal("{{ .other }}"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("spec.deployment.extension.values"),
					"BadValue": Equal("{{ .resources.bad-name.data.k }}"),
				})),
			))
		})

		It("should reject a template referencing an undeclared resource name", func() {
			extension().Spec.Deployment.ExtensionDeployment.Values = &apiextensionsv1.JSON{Raw: []byte(`{"x":"{{ .resources.undeclared.data.token }}"}`)}
			Expect(test()).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("spec.deployment.extension.values"),
				"Detail": ContainSubstring(`template references resource "undeclared" which is not declared in the resources list`),
			}))))
		})

		It("should accept values without any template expressions", func() {
			extension().Spec.Deployment.ExtensionDeployment.Values = &apiextensionsv1.JSON{Raw: []byte(`{"x":"plain string","y":42}`)}
			Expect(test()).To(BeEmpty())
		})
	})
}
