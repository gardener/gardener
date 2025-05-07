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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/validation"
)

var _ = Describe("validation", func() {
	var backupBucket *core.BackupBucket

	BeforeEach(func() {
		seed := "some-seed"
		backupBucket = &core.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: "example-backup-bucket",
			},
			Spec: core.BackupBucketSpec{
				Provider: core.BackupBucketProvider{
					Type:   "some-provider",
					Region: "some-region",
				},
				CredentialsRef: &corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  "garden",
					Name:       "backup-secret",
				},
				SecretRef: corev1.SecretReference{
					Name:      "backup-secret",
					Namespace: "garden",
				},
				SeedName: &seed,
			},
		}
	})

	Describe("#ValidateBackupBucket", func() {
		It("should not return any errors", func() {
			errorList := ValidateBackupBucket(backupBucket)

			Expect(errorList).To(BeEmpty())
		})

		DescribeTable("BackupBucket metadata",
			func(objectMeta metav1.ObjectMeta, matcher gomegatypes.GomegaMatcher) {
				backupBucket.ObjectMeta = objectMeta

				errorList := ValidateBackupBucket(backupBucket)

				Expect(errorList).To(matcher)
			},

			Entry("should forbid BackupBucket with empty metadata",
				metav1.ObjectMeta{},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid BackupBucket with empty name",
				metav1.ObjectMeta{Name: ""},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid BackupBucket with '.' in the name (not a DNS-1123 label compliant name)",
				metav1.ObjectMeta{Name: "backup-bucket.test"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid BackupBucket with '_' in the name (not a DNS-1123 subdomain)",
				metav1.ObjectMeta{Name: "backup-bucket_test"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
		)

		It("should forbid BackupBucket specification with empty or invalid keys", func() {
			backupBucket.Spec.Provider.Type = ""
			backupBucket.Spec.Provider.Region = ""
			backupBucket.Spec.SecretRef = corev1.SecretReference{}
			backupBucket.Spec.CredentialsRef = nil
			backupBucket.Spec.SeedName = nil

			errorList := ValidateBackupBucket(backupBucket)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.provider.type"),
			})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.provider.region"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("spec.credentialsRef"),
					"Detail": Equal(`must be set to refer a Secret or WorkloadIdentity`),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.seedName"),
				}))))
		})

		It("should forbid updating some keys", func() {
			newBackupBucket := prepareBackupBucketForUpdate(backupBucket)
			newBackupBucket.Spec.Provider.Type = "another-type"
			newBackupBucket.Spec.Provider.Region = "another-region"
			seed := "another-seed"
			newBackupBucket.Spec.SeedName = &seed

			errorList := ValidateBackupBucketUpdate(newBackupBucket, backupBucket)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.provider"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.seedName"),
				})),
			))
		})

		Context("backup credentialsRef and secretRef", func() {
			It("should require credentialsRef to be set", func() {
				backupBucket.Spec.CredentialsRef = nil

				Expect(ValidateBackupBucket(backupBucket)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("spec.credentialsRef"),
						"Detail": Equal("must be set to refer a Secret or WorkloadIdentity"),
					})),
				))
			})

			It("should forbid credentialsRef to refer a WorkloadIdentity", func() {
				backupBucket.Spec.CredentialsRef = &corev1.ObjectReference{APIVersion: "security.gardener.cloud/v1alpha1", Kind: "WorkloadIdentity", Namespace: "garden", Name: "backup"}
				backupBucket.Spec.SecretRef = corev1.SecretReference{}

				Expect(ValidateBackupBucket(backupBucket)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.credentialsRef"),
						"Detail": Equal("support for workload identity as backup credentials is not yet fully implemented"),
					})),
				))
			})

			It("should allow credentialsRef to refer a Secret", func() {
				backupBucket.Spec.CredentialsRef = &corev1.ObjectReference{APIVersion: "v1", Kind: "Secret", Namespace: "garden", Name: "backup"}
				backupBucket.Spec.SecretRef = corev1.SecretReference{
					Namespace: backupBucket.Spec.CredentialsRef.Namespace,
					Name:      backupBucket.Spec.CredentialsRef.Name,
				}

				Expect(ValidateBackupBucket(backupBucket)).To((BeEmpty()))
			})

			It("should forbid invalid values objectReference fields", func() {
				backupBucket.Spec.CredentialsRef = &corev1.ObjectReference{APIVersion: "", Kind: "", Namespace: "", Name: ""}
				backupBucket.Spec.SecretRef = corev1.SecretReference{}

				Expect(ValidateBackupBucket(backupBucket)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("spec.credentialsRef.apiVersion"),
						"Detail": Equal("must provide an apiVersion"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("spec.credentialsRef.kind"),
						"Detail": Equal("must provide a kind"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("spec.credentialsRef.name"),
						"Detail": Equal("must provide a name"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.credentialsRef.name"),
						"Detail": ContainSubstring("a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("spec.credentialsRef.namespace"),
						"Detail": Equal("must provide a namespace"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.credentialsRef.namespace"),
						"Detail": ContainSubstring("a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeNotSupported),
						"Field":  Equal("spec.credentialsRef"),
						"Detail": Equal(`supported values: "/v1, Kind=Secret", "security.gardener.cloud/v1alpha1, Kind=WorkloadIdentity"`),
					})),
				))
			})

			It("should forbid secretRef to refer a Secret other than the one referred by the credentialsRef", func() {
				backupBucket.Spec.CredentialsRef = &corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  "garden",
					Name:       "backup-secret",
				}
				backupBucket.Spec.SecretRef = corev1.SecretReference{
					Namespace: "another-namespace",
					Name:      "another-secret",
				}

				Expect(ValidateBackupBucket(backupBucket)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.secretRef"),
						"Detail": Equal("must refer to the same secret as `spec.credentialsRef`"),
					})),
				))
			})

			It("should forbid secretRef to be empty when credentialsRef refer a Secret", func() {
				backupBucket.Spec.CredentialsRef = &corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
					Namespace:  "garden",
					Name:       "backup-secret",
				}
				backupBucket.Spec.SecretRef = corev1.SecretReference{}

				Expect(ValidateBackupBucket(backupBucket)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.secretRef"),
						"Detail": Equal("must refer to the same secret as `spec.credentialsRef`"),
					})),
				))
			})

			It("should forbid secretRef to be set when credentialsRef refer a WorkloadIdentity", func() {
				backupBucket.Spec.CredentialsRef = &corev1.ObjectReference{
					APIVersion: "security.gardener.cloud/v1alpha1",
					Kind:       "WorkloadIdentity",
					Namespace:  "garden",
					Name:       "backup-secret",
				}
				backupBucket.Spec.SecretRef = corev1.SecretReference{Namespace: "foo", Name: "bar"}

				Expect(ValidateBackupBucket(backupBucket)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.secretRef"),
						"Detail": Equal("must not be set when `spec.credentialsRef` refer to resource other than secret"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.credentialsRef"),
						"Detail": Equal("support for workload identity as backup credentials is not yet fully implemented"),
					})),
				))
			})
		})
	})

})

func prepareBackupBucketForUpdate(obj *core.BackupBucket) *core.BackupBucket {
	newObj := obj.DeepCopy()
	newObj.ResourceVersion = "1"
	return newObj
}
