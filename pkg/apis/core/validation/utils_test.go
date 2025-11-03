// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/validation"
)

var _ = Describe("Utils tests", func() {
	Describe("#ValidateFailureToleranceTypeValue", func() {
		var fldPath *field.Path

		BeforeEach(func() {
			fldPath = field.NewPath("spec", "highAvailability", "failureTolerance", "type")
		})

		It("highAvailability is set to failureTolerance of node", func() {
			errorList := ValidateFailureToleranceTypeValue(core.FailureToleranceTypeNode, fldPath)
			Expect(errorList).To(BeEmpty())
		})

		It("highAvailability is set to failureTolerance of zone", func() {
			errorList := ValidateFailureToleranceTypeValue(core.FailureToleranceTypeZone, fldPath)
			Expect(errorList).To(BeEmpty())
		})

		It("highAvailability is set to an unsupported value", func() {
			errorList := ValidateFailureToleranceTypeValue("region", fldPath)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal(fldPath.String()),
				}))))
		})
	})

	Describe("#ValidateIPFamilies", func() {
		var fldPath *field.Path

		BeforeEach(func() {
			fldPath = field.NewPath("ipFamilies")
		})

		It("should deny unsupported IP families", func() {
			errorList := ValidateIPFamilies([]core.IPFamily{"foo", "bar"}, fldPath)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeNotSupported),
					"Field":    Equal(fldPath.Index(0).String()),
					"BadValue": BeEquivalentTo("foo"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeNotSupported),
					"Field":    Equal(fldPath.Index(1).String()),
					"BadValue": BeEquivalentTo("bar"),
				})),
			))
		})

		It("should deny duplicate IP families", func() {
			errorList := ValidateIPFamilies([]core.IPFamily{core.IPFamilyIPv4, core.IPFamilyIPv6, core.IPFamilyIPv4, core.IPFamilyIPv6}, fldPath)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeDuplicate),
					"Field":    Equal(fldPath.Index(2).String()),
					"BadValue": Equal(core.IPFamilyIPv4),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeDuplicate),
					"Field":    Equal(fldPath.Index(3).String()),
					"BadValue": Equal(core.IPFamilyIPv6),
				})),
			))
		})

		It("should allow IPv4 single-stack", func() {
			errorList := ValidateIPFamilies([]core.IPFamily{core.IPFamilyIPv4}, fldPath)
			Expect(errorList).To(BeEmpty())
		})

		It("should allow IPv6 single-stack", func() {
			errorList := ValidateIPFamilies([]core.IPFamily{core.IPFamilyIPv6}, fldPath)
			Expect(errorList).To(BeEmpty())
		})
	})

	Describe("#ValidateObjectReferenceNameAndNamespace", func() {
		var fldPath *field.Path

		BeforeEach(func() {
			fldPath = field.NewPath("objectRef")
		})

		It("should allow only name when namespace is not required", func() {
			ref := corev1.ObjectReference{Name: "name"}
			Expect(ValidateObjectReferenceNameAndNamespace(ref, fldPath, false)).To(BeEmpty())
		})

		It("should allow name and namespace when namespace is not required", func() {
			ref := corev1.ObjectReference{Name: "name", Namespace: "namespace"}
			Expect(ValidateObjectReferenceNameAndNamespace(ref, fldPath, false)).To(BeEmpty())
		})

		It("should allow name and namespace when namespace is required", func() {
			ref := corev1.ObjectReference{Name: "name", Namespace: "namespace"}
			Expect(ValidateObjectReferenceNameAndNamespace(ref, fldPath, true)).To(BeEmpty())
		})

		It("should deny unset name when namespace is not required and unset", func() {
			ref := corev1.ObjectReference{}
			Expect(ValidateObjectReferenceNameAndNamespace(ref, fldPath, false)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fldPath.Child("name").String()),
				})),
			))
		})

		It("should deny unset name when namespace is not required but set", func() {
			ref := corev1.ObjectReference{Namespace: "namespace"}
			Expect(ValidateObjectReferenceNameAndNamespace(ref, fldPath, false)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fldPath.Child("name").String()),
				})),
			))
		})

		It("should deny unset name and namespace when namespace is required", func() {
			ref := corev1.ObjectReference{}
			Expect(ValidateObjectReferenceNameAndNamespace(ref, fldPath, true)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fldPath.Child("name").String()),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fldPath.Child("namespace").String()),
				})),
			))
		})

		It("should deny unset name when namespace is required and set", func() {
			ref := corev1.ObjectReference{Namespace: "namespace"}
			Expect(ValidateObjectReferenceNameAndNamespace(ref, fldPath, true)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fldPath.Child("name").String()),
				})),
			))
		})

		It("should deny a non-DNS1123 name", func() {
			ref := corev1.ObjectReference{Name: "-name-"}
			Expect(ValidateObjectReferenceNameAndNamespace(ref, fldPath, false)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(fldPath.Child("name").String()),
					"BadValue": Equal("-name-"),
				})),
			))
		})

		It("should deny an invalid namespace", func() {
			ref := corev1.ObjectReference{Name: "name", Namespace: "namespace-123-@"}
			Expect(ValidateObjectReferenceNameAndNamespace(ref, fldPath, true)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(fldPath.Child("namespace").String()),
					"BadValue": Equal("namespace-123-@"),
				})),
			))
		})
	})

	DescribeTable("#ValidateCredentialsRef",
		func(ref corev1.ObjectReference, matcher gomegatypes.GomegaMatcher) {
			fldPath := field.NewPath("credentialsRef")
			errList := ValidateCredentialsRef(ref, fldPath)
			Expect(errList).To(matcher)
		},
		Entry("should allow v1.Secret",
			corev1.ObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo", Namespace: "bar"},
			BeEmpty(),
		),
		Entry("should allow security.gardener.cloud/v1alpha1.WorkloadIdentity",
			corev1.ObjectReference{APIVersion: "security.gardener.cloud/v1alpha1", Kind: "WorkloadIdentity", Name: "foo", Namespace: "bar"},
			BeEmpty(),
		),
		Entry("should forbid v1.Secret with non DNS1123 name",
			corev1.ObjectReference{APIVersion: "v1", Kind: "Secret", Name: "Foo", Namespace: "bar"},
			ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("credentialsRef.name"),
					"BadValue": Equal("Foo"),
				})),
			),
		),
		Entry("should forbid security.gardener.cloud/v1alpha1.WorkloadIdentity with non DNS1123 namespace",
			corev1.ObjectReference{APIVersion: "security.gardener.cloud/v1alpha1", Kind: "WorkloadIdentity", Name: "foo", Namespace: "bar?"},
			ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("credentialsRef.namespace"),
					"BadValue": Equal("bar?"),
				})),
			),
		),
		Entry("should forbid credentialsRef with empty apiVersion, kind, name, or namespace",
			corev1.ObjectReference{APIVersion: "", Kind: "", Name: "", Namespace: ""},
			ContainElements(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("credentialsRef.apiVersion"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("credentialsRef.kind"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("credentialsRef.name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("credentialsRef.namespace"),
				})),
			),
		),
		Entry("should forbid v1.ConfigMap",
			corev1.ObjectReference{APIVersion: "v1", Kind: "ConfigMap", Name: "foo", Namespace: "bar"},
			ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("credentialsRef"),
				})),
			),
		),
		Entry("should forbid security.gardener.cloud/v1alpha1.FooBar",
			corev1.ObjectReference{APIVersion: "security.gardener.cloud/v1alpha1", Kind: "FooBar", Name: "foo", Namespace: "bar"},
			ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("credentialsRef"),
				})),
			),
		),
		Entry("should forbid security.gardener.cloud/v2alpha1.WorkloadIdentity",
			corev1.ObjectReference{APIVersion: "security.gardener.cloud/v2alpha1", Kind: "WorkloadIdentity", Name: "foo", Namespace: "bar"},
			ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("credentialsRef"),
				})),
			),
		),
	)

	Describe("#ValidateMachineImages", func() {
		DescribeTable("should not allow invalid machine image names",
			func(name string, shouldFail bool) {
				validationResult := ValidateMachineImages([]core.MachineImage{{Name: name}}, nil, field.NewPath("spec", "machineImages"), true)

				if shouldFail {
					Expect(validationResult).
						To(ConsistOf(
							PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":     Equal(field.ErrorTypeInvalid),
								"Field":    Equal("spec.machineImages[0].name"),
								"BadValue": Equal(name),
								"Detail":   ContainSubstring("machine image name must be a qualified name"),
							})),
						))
				} else {
					Expect(validationResult).To(BeEmpty())
				}
			},
			Entry("forbid emoji characters", "🪴", true),
			Entry("forbid whitespaces", "special image", true),
			Entry("forbid slashes", "nested/image", true),
			Entry("pass with dashes", "qualified-name", false),
		)
	})

	Describe("#ValidateCloudProfileSpec", func() {
		var specTemplate = &core.CloudProfileSpec{
			Type:    "local",
			Regions: []core.Region{{Name: "local"}},
			Kubernetes: core.KubernetesSettings{
				Versions: []core.ExpirableVersion{
					{Version: "1.0.0"},
				},
			},
			MachineImages: []core.MachineImage{
				{
					Name: "test-image",
					Versions: []core.MachineImageVersion{{
						ExpirableVersion: core.ExpirableVersion{Version: "1.0.0"},
						CRI:              []core.CRI{{Name: "containerd"}},
						Architectures:    []string{"amd64"},
					}},
					UpdateStrategy: ptr.To(core.UpdateStrategyMajor),
				},
			},
			MachineTypes: []core.MachineType{{Name: "valid", Architecture: ptr.To("amd64")}},
			VolumeTypes:  []core.VolumeType{{Class: "standard", Name: "valid"}},
		}

		DescribeTable("should not allow invalid machine type names",
			func(name string, shouldFail bool) {
				spec := specTemplate.DeepCopy()
				spec.MachineTypes[0].Name = name

				validationResult := ValidateCloudProfileSpec(spec, field.NewPath("spec"))

				if shouldFail {
					Expect(validationResult).
						To(ConsistOf(
							PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":     Equal(field.ErrorTypeInvalid),
								"Field":    Equal("spec.machineTypes[0].name"),
								"BadValue": Equal(name),
								"Detail":   ContainSubstring("machine type name must be a qualified name"),
							})),
						))
				} else {
					Expect(validationResult).To(BeEmpty())
				}
			},
			Entry("forbid emoji characters", "🪴", true),
			Entry("forbid whitespaces", "special image", true),
			Entry("forbid slashes", "nested/image", true),
			Entry("pass with dashes and dots", "a.qualified-name", false),
		)

		DescribeTable("should not allow invalid volume type names",
			func(name string, shouldFail bool) {
				spec := specTemplate.DeepCopy()
				spec.VolumeTypes[0].Name = name

				validationResult := ValidateCloudProfileSpec(spec, field.NewPath("spec"))

				if shouldFail {
					Expect(validationResult).
						To(ConsistOf(
							PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":     Equal(field.ErrorTypeInvalid),
								"Field":    Equal("spec.volumeTypes[0].name"),
								"BadValue": Equal(name),
								"Detail":   ContainSubstring("volume type name must be a qualified name"),
							})),
						))
				} else {
					Expect(validationResult).To(BeEmpty())
				}
			},
			Entry("forbid emoji characters", "🪴", true),
			Entry("forbid whitespaces", "special image", true),
			Entry("forbid slashes", "nested/image", true),
			Entry("pass with dashes and dots", "a.qualified-name", false),
		)

		DescribeTable("should not allow invalid volume type class",
			func(name string, shouldFail bool) {
				spec := specTemplate.DeepCopy()
				spec.VolumeTypes[0].Class = name

				validationResult := ValidateCloudProfileSpec(spec, field.NewPath("spec"))

				if shouldFail {
					Expect(validationResult).
						To(ConsistOf(
							PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.volumeTypes[0].class"),
								"Detail": ContainSubstring("volume class must be a qualified name"),
							})),
						))
				} else {
					Expect(validationResult).To(BeEmpty())
				}
			},
			Entry("forbid emoji characters", "🪴", true),
			Entry("forbid whitespaces", "special image", true),
			Entry("forbid slashes", "nested/image", true),
			Entry("pass with dashes and dots", "a.qualified-name", false),
		)
	})
})
