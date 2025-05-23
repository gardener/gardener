// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/security"
	. "github.com/gardener/gardener/pkg/apis/security/validation"
)

var originalPrefixAndDelimiterFunc func() (string, string)
var _ = BeforeSuite(func() {
	originalPrefixAndDelimiterFunc = GetSubClaimPrefixAndDelimiterFunc
})

var _ = Describe("WorkloadIdentity Validation Tests", func() {
	var workloadIdentity *security.WorkloadIdentity

	BeforeEach(func() {
		workloadIdentity = &security.WorkloadIdentity{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "identity",
				Namespace: "garden",
				UID:       "b0f6d68e-aae4-483e-aab1-eab98d9e608c",
			},
			Spec: security.WorkloadIdentitySpec{
				Audiences: []string{"gardener.cloud"},
				TargetSystem: security.TargetSystem{
					Type:           "foo",
					ProviderConfig: nil,
				},
			},
			Status: security.WorkloadIdentityStatus{
				Sub: "gardener.cloud:workloadidentity:garden:identity:b0f6d68e-aae4-483e-aab1-eab98d9e608c",
			},
		}
		GetSubClaimPrefixAndDelimiterFunc = originalPrefixAndDelimiterFunc
	})

	Describe("#ValidateWorkloadIdentity", func() {
		It("should not return any errors", func() {
			errorList := ValidateWorkloadIdentity(workloadIdentity)

			Expect(errorList).To(BeEmpty())
		})

		DescribeTable("WorkloadIdentity metadata",
			func(objectMeta metav1.ObjectMeta, matcher gomegatypes.GomegaMatcher) {
				workloadIdentity.ObjectMeta = objectMeta
				p, d := GetSubClaimPrefixAndDelimiterFunc()
				workloadIdentity.Status.Sub = strings.Join([]string{p, objectMeta.GetNamespace(), objectMeta.GetName(), string(objectMeta.GetUID())}, d)

				errorList := ValidateWorkloadIdentity(workloadIdentity)

				Expect(errorList).To(matcher)
			},

			Entry("should forbid WorkloadIdentity with empty metadata",
				metav1.ObjectMeta{},
				ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("metadata.name"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("metadata.namespace"),
					})),
				),
			),
			Entry("should forbid WorkloadIdentity with empty name",
				metav1.ObjectMeta{Name: "", Namespace: "garden"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should allow WorkloadIdentity with '.' in the name",
				metav1.ObjectMeta{Name: "binding.test", Namespace: "garden"},
				BeEmpty(),
			),
			Entry("should forbid WorkloadIdentity with '_' in the name (not a DNS-1123 subdomain)",
				metav1.ObjectMeta{Name: "binding_test", Namespace: "garden"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
		)

		It("should forbid empty WorkloadIdentity resources", func() {
			workloadIdentity.ObjectMeta = metav1.ObjectMeta{}
			workloadIdentity.Spec = security.WorkloadIdentitySpec{}
			workloadIdentity.Status = security.WorkloadIdentityStatus{}

			errorList := ValidateWorkloadIdentity(workloadIdentity)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.namespace"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.targetSystem.type"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.audiences"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("status.sub"),
					"Detail": Equal("must specify sub claim value"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("status.sub"),
					"Detail": Equal(`sub claim does not match expected value: "gardener.cloud:workloadidentity:::"`),
				})),
			))
		})

		DescribeTable("Audiences",
			func(audiences []string, matcher gomegatypes.GomegaMatcher) {
				workloadIdentity.Spec.Audiences = audiences
				errList := ValidateWorkloadIdentity(workloadIdentity)
				Expect(errList).To(matcher)
			},
			Entry("should allow single non-empty audience",
				[]string{"foo"},
				BeEmpty(),
			),
			Entry("should allow multiple non-empty audience",
				[]string{"foo", "bar"},
				BeEmpty(),
			),
			Entry("should forbid no audience",
				[]string{},
				ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("spec.audiences"),
						"Detail": Equal("must provide at least one audience"),
					})),
				),
			),
			Entry("should forbid single empty audience",
				[]string{""},
				ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.audiences[0]"),
					})),
				),
			),
			Entry("should forbid multiple empty audience",
				[]string{"", "", ""},
				ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.audiences[0]"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.audiences[1]"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal("spec.audiences[1]"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.audiences[2]"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal("spec.audiences[2]"),
					})),
				),
			),
			Entry("should forbid empty audience",
				[]string{"foo", "", "bar"},
				ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.audiences[1]"),
					})),
				),
			),
			Entry("should forbid duplicated audience",
				[]string{"foo", "bar", "bar"},
				ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeDuplicate),
						"Field":  Equal("spec.audiences[2]"),
						"Detail": Equal(""),
					})),
				),
			),
		)

		DescribeTable("TargetSystem",
			func(targetSystem security.TargetSystem, matcher gomegatypes.GomegaMatcher) {
				workloadIdentity.Spec.TargetSystem = targetSystem
				errList := ValidateWorkloadIdentity(workloadIdentity)
				Expect(errList).To(matcher)
			},
			Entry("should allow valid target system type",
				security.TargetSystem{Type: "foo"},
				BeEmpty(),
			),
			Entry("should forbid empty target system type",
				security.TargetSystem{Type: ""},
				ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.targetSystem.type"),
					})),
				),
			),
			Entry("should forbid multiple target system types",
				security.TargetSystem{Type: "foo,bar"},
				ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.targetSystem.type"),
					})),
				),
			),
		)

		DescribeTable("Sub claim",
			func(name string, f func() (string, string), matcher gomegatypes.GomegaMatcher) {
				workloadIdentity.Name = name

				if f != nil {
					GetSubClaimPrefixAndDelimiterFunc = f
				}
				p, d := GetSubClaimPrefixAndDelimiterFunc()
				workloadIdentity.Status.Sub = strings.Join([]string{p, workloadIdentity.Namespace, workloadIdentity.Name, string(workloadIdentity.UID)}, d)

				errList := ValidateWorkloadIdentity(workloadIdentity)
				Expect(errList).To(matcher)
			},
			Entry("should allow sub claim value containing only ascii chars shorter than 255 symbols",
				"my-name", nil,
				BeEmpty(),
			),
			Entry("should allow sub claim value containing only ascii chars 255 symbols long",
				strings.Repeat("x", 179), nil, // 255 - len("garden"=6) - len(uid=36) - 3*1(delimiter) - len(prefix="workloadidentity:gardener.cloud"=31) = 179
				BeEmpty(),
			),
			Entry("should forbid sub claim value containing only ascii chars longer than 255 symbols",
				strings.Repeat("x", 180), nil, // 256 - len("garden"=6) - len(uid=36) - 3*1(delimiter) - len(prefix="workloadidentity:gardener.cloud"=31) = 180
				ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("status.sub"),
					})),
				),
			),
			Entry("should forbid non-ascii chars sub claim",
				"my-name", func() (string, string) {
					return "Ω", ":"
				},
				ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("status.sub"),
						"Detail": ContainSubstring(`sub claim contains non-ascii symbol('Ω') at index 0`),
					})),
				),
			),
		)

		It("should forbid sub value to not match expected value", func() {
			workloadIdentity.Status.Sub = "foo"
			errList := ValidateWorkloadIdentity(workloadIdentity)
			Expect(errList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("status.sub"),
					"Detail": Equal(`sub claim does not match expected value: "gardener.cloud:workloadidentity:garden:identity:b0f6d68e-aae4-483e-aab1-eab98d9e608c"`),
				})),
			))
		})

	})

	Describe("#ValidateWorkloadIdentityUpdate", func() {
		It("should forbid updating the WorkloadIdentity provider type when the field is already set", func() {
			newWorkloadIdentity := prepareWorkloadIdentityForUpdate(workloadIdentity)
			newWorkloadIdentity.Spec.TargetSystem.Type = "new-type"

			errorList := ValidateWorkloadIdentityUpdate(newWorkloadIdentity, workloadIdentity)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.targetSystem.type"),
				})),
			))
		})

		It("should forbid updating the WorkloadIdentity sub claim value when the field is already set", func() {
			newWorkloadIdentity := prepareWorkloadIdentityForUpdate(workloadIdentity)
			newWorkloadIdentity.Status.Sub = "new-sub"

			errorList := ValidateWorkloadIdentityUpdate(newWorkloadIdentity, workloadIdentity)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("status.sub"),
					"Detail": Equal("field is immutable"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("status.sub"),
					"Detail": ContainSubstring("sub claim does not match expected value: "),
				})),
			))
		})
	})

})

func prepareWorkloadIdentityForUpdate(workloadIdentity *security.WorkloadIdentity) *security.WorkloadIdentity {
	c := workloadIdentity.DeepCopy()
	c.ResourceVersion = "1"
	return c
}
