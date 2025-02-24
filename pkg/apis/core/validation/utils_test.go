// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("credentialsRef.name"),
				})),
			),
		),
		Entry("should forbid security.gardener.cloud/v1alpha1.WorkloadIdentity with non DNS1123 namespace",
			corev1.ObjectReference{APIVersion: "security.gardener.cloud/v1alpha1", Kind: "WorkloadIdentity", Name: "foo", Namespace: "bar?"},
			ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("credentialsRef.namespace"),
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
})
