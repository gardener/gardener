// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

var _ = Describe("Validation Tests", func() {
	Describe("#ValidateExtension", func() {
		var extension *operatorv1alpha1.Extension

		BeforeEach(func() {
			extension = &operatorv1alpha1.Extension{
				Spec: operatorv1alpha1.ExtensionSpec{
					Deployment: &operatorv1alpha1.Deployment{
						ExtensionDeployment: &operatorv1alpha1.ExtensionDeploymentSpec{
							DeploymentSpec: operatorv1alpha1.DeploymentSpec{
								Helm: &operatorv1alpha1.ExtensionHelm{
									OCIRepository: &gardencorev1.OCIRepository{
										Ref: ptr.To("example.com/chart:v1.0.0"),
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

		It("should return no errors for basic extension", func() {
			Expect(ValidateExtension(extension)).To(BeEmpty())
		})

		It("should return an error when deployment is nil", func() {
			extension.Spec.Deployment = nil

			Expect(ValidateExtension(extension)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.deployment"),
			}))))
		})

		It("should return an error when neither extension nor admission deployment is specified", func() {
			extension.Spec.Deployment.ExtensionDeployment = nil
			extension.Spec.Deployment.AdmissionDeployment = nil

			Expect(ValidateExtension(extension)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.deployment"),
			}))))
		})

		Context("Extension Deployment", func() {
			It("should return no errors when extension deployment is nil", func() {
				extension.Spec.Deployment.ExtensionDeployment = nil
				extension.Spec.Deployment.AdmissionDeployment = &operatorv1alpha1.AdmissionDeploymentSpec{}

				Expect(ValidateExtension(extension)).To(BeEmpty())
			})

			It("should return an error when extension deployment has no helm config", func() {
				extension.Spec.Deployment.ExtensionDeployment = &operatorv1alpha1.ExtensionDeploymentSpec{}

				Expect(ValidateExtension(extension)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.deployment.extension.helm"),
				}))))
			})

			It("should return an error when admission runtime deployment has no helm config", func() {
				extension.Spec.Deployment.AdmissionDeployment = &operatorv1alpha1.AdmissionDeploymentSpec{
					RuntimeCluster: &operatorv1alpha1.DeploymentSpec{},
				}

				Expect(ValidateExtension(extension)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.deployment.admission.runtimeCluster.helm"),
				}))))
			})

			It("should return an error when admission virtual deployment has no helm config", func() {
				extension.Spec.Deployment.AdmissionDeployment = &operatorv1alpha1.AdmissionDeploymentSpec{
					VirtualCluster: &operatorv1alpha1.DeploymentSpec{},
				}

				Expect(ValidateExtension(extension)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.deployment.admission.virtualCluster.helm"),
				}))))
			})

			It("should return an error when extension deployment helm has invalid OCI repository", func() {
				extension.Spec.Deployment.ExtensionDeployment = &operatorv1alpha1.ExtensionDeploymentSpec{
					DeploymentSpec: operatorv1alpha1.DeploymentSpec{
						Helm: &operatorv1alpha1.ExtensionHelm{
							OCIRepository: &gardencorev1.OCIRepository{},
						},
					},
				}

				Expect(ValidateExtension(extension)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.deployment.extension.helm.ociRepository"),
				}))))
			})

			It("should return no errors when extension deployment helm has valid OCI repository with ref", func() {
				extension.Spec.Deployment.ExtensionDeployment = &operatorv1alpha1.ExtensionDeploymentSpec{
					DeploymentSpec: operatorv1alpha1.DeploymentSpec{
						Helm: &operatorv1alpha1.ExtensionHelm{
							OCIRepository: &gardencorev1.OCIRepository{
								Ref: ptr.To("example.com/chart:v1.0.0"),
							},
						},
					},
				}

				Expect(ValidateExtension(extension)).To(BeEmpty())
			})

			It("should return no errors when extension deployment helm has valid OCI repository with repository and tag", func() {
				extension.Spec.Deployment.ExtensionDeployment = &operatorv1alpha1.ExtensionDeploymentSpec{
					DeploymentSpec: operatorv1alpha1.DeploymentSpec{
						Helm: &operatorv1alpha1.ExtensionHelm{
							OCIRepository: &gardencorev1.OCIRepository{
								Repository: ptr.To("example.com/chart"),
								Tag:        ptr.To("v1.0.0"),
							},
						},
					},
				}

				Expect(ValidateExtension(extension)).To(BeEmpty())
			})
		})

		Context("Admission Deployment", func() {
			It("should return no errors when admission deployment is nil", func() {
				extension.Spec.Deployment.AdmissionDeployment = nil

				Expect(ValidateExtension(extension)).To(BeEmpty())
			})

			It("should return an error when admission runtime cluster has invalid OCI repository", func() {
				extension.Spec.Deployment.AdmissionDeployment = &operatorv1alpha1.AdmissionDeploymentSpec{
					RuntimeCluster: &operatorv1alpha1.DeploymentSpec{
						Helm: &operatorv1alpha1.ExtensionHelm{
							OCIRepository: &gardencorev1.OCIRepository{},
						},
					},
				}

				Expect(ValidateExtension(extension)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.deployment.admission.runtimeCluster.helm.ociRepository"),
				}))))
			})

			It("should return no errors when admission runtime cluster has valid OCI repository", func() {
				extension.Spec.Deployment.AdmissionDeployment = &operatorv1alpha1.AdmissionDeploymentSpec{
					RuntimeCluster: &operatorv1alpha1.DeploymentSpec{
						Helm: &operatorv1alpha1.ExtensionHelm{
							OCIRepository: &gardencorev1.OCIRepository{
								Ref: ptr.To("example.com/admission:v1.0.0"),
							},
						},
					},
				}

				Expect(ValidateExtension(extension)).To(BeEmpty())
			})

			It("should return an error when admission virtual cluster has invalid OCI repository", func() {
				extension.Spec.Deployment.AdmissionDeployment = &operatorv1alpha1.AdmissionDeploymentSpec{
					VirtualCluster: &operatorv1alpha1.DeploymentSpec{
						Helm: &operatorv1alpha1.ExtensionHelm{
							OCIRepository: &gardencorev1.OCIRepository{},
						},
					},
				}

				Expect(ValidateExtension(extension)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.deployment.admission.virtualCluster.helm.ociRepository"),
				}))))
			})

			It("should return no errors when admission virtual cluster has valid OCI repository", func() {
				extension.Spec.Deployment.AdmissionDeployment = &operatorv1alpha1.AdmissionDeploymentSpec{
					VirtualCluster: &operatorv1alpha1.DeploymentSpec{
						Helm: &operatorv1alpha1.ExtensionHelm{
							OCIRepository: &gardencorev1.OCIRepository{
								Repository: ptr.To("example.com/admission"),
								Digest:     ptr.To("sha256:abc123"),
							},
						},
					},
				}

				Expect(ValidateExtension(extension)).To(BeEmpty())
			})

			It("should return errors when both runtime and virtual clusters have invalid OCI repositories", func() {
				extension.Spec.Deployment.AdmissionDeployment = &operatorv1alpha1.AdmissionDeploymentSpec{
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

				errs := ValidateExtension(extension)
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
	})

	Describe("#ValidateExtensionUpdate", func() {
		var extension *operatorv1alpha1.Extension

		BeforeEach(func() {
			extension = &operatorv1alpha1.Extension{
				Spec: operatorv1alpha1.ExtensionSpec{
					Deployment: &operatorv1alpha1.Deployment{},
				},
			}
		})

		It("should return no errors for basic extension", func() {
			newExtension := extension.DeepCopy()
			newExtension.Spec.Deployment.ExtensionDeployment = &operatorv1alpha1.ExtensionDeploymentSpec{
				DeploymentSpec: operatorv1alpha1.DeploymentSpec{
					Helm: &operatorv1alpha1.ExtensionHelm{
						OCIRepository: &gardencorev1.OCIRepository{
							Ref: ptr.To("example.com/chart:v1.0.0"),
						},
					},
				},
			}

			newExtension.Spec.Resources = []gardencorev1beta1.ControllerResource{
				{Kind: "Extension", Type: "type-a"},
			}

			Expect(ValidateExtensionUpdate(extension, newExtension)).To(BeEmpty())
		})

		It("should return an error when extension is nil", func() {
			extension.Spec.Deployment = nil

			Expect(ValidateExtensionUpdate(extension, extension)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.deployment"),
			}))))
		})

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
										Ref: ptr.To("example.com/chart:v1.0.0"),
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
				test(ptr.To(true), ptr.To(true), BeEmpty())
			})

			It("should return no errors when field is set from nil to true", func() {
				test(nil, ptr.To(true), BeEmpty())
			})

			It("should return an error because the primary field is changed to false", func() {
				test(ptr.To(true), ptr.To(false), ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.resources[0].primary"),
				}))))
			})

			It("should return an error because the primary field is changed to nil", func() {
				test(ptr.To(false), nil, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.resources[0].primary"),
				}))))
			})

			It("should return an error because the primary field is changed to true", func() {
				test(ptr.To(false), ptr.To(true), ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.resources[0].primary"),
				}))))
			})
		})
	})
})
