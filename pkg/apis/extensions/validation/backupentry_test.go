// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/extensions/validation"
)

var _ = Describe("BackupEntry validation tests", func() {
	var be *extensionsv1alpha1.BackupEntry

	BeforeEach(func() {
		be = &extensionsv1alpha1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-be",
			},
			Spec: extensionsv1alpha1.BackupEntrySpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: "provider",
				},
				BucketName: "bucket-name",
				Region:     "region",
				SecretRef: corev1.SecretReference{
					Name: "test",
				},
			},
		}
	})

	Describe("#ValidBackupEntry", func() {
		It("should forbid empty BackupEntry resources", func() {
			errorList := ValidateBackupEntry(&extensionsv1alpha1.BackupEntry{})

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.type"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.region"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.bucketName"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.secretRef.name"),
			}))))
		})

		It("should allow valid be resources", func() {
			errorList := ValidateBackupEntry(be)

			Expect(errorList).To(BeEmpty())
		})
	})

	Describe("#ValidBackupEntryUpdate", func() {
		It("should prevent updating anything if deletion time stamp is set", func() {
			now := metav1.Now()
			be.DeletionTimestamp = &now

			newBackupEntry := prepareBackupEntryForUpdate(be)
			newBackupEntry.DeletionTimestamp = &now
			newBackupEntry.Spec.SecretRef.Name = "changed-secretref-name"

			errorList := ValidateBackupEntryUpdate(newBackupEntry, be)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec"),
				"Detail": Equal("cannot update backup entry spec if deletion timestamp is set. Requested changes: SecretRef.Name: changed-secretref-name != test"),
			}))))
		})

		It("should prevent updating the type", func() {
			newBackupEntry := prepareBackupEntryForUpdate(be)
			newBackupEntry.Spec.Type = "changed-type"

			errorList := ValidateBackupEntryUpdate(newBackupEntry, be)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.type"),
			}))))
		})

		It("should allow updating the name of the referenced secret", func() {
			newBackupEntry := prepareBackupEntryForUpdate(be)
			newBackupEntry.Spec.SecretRef.Name = "changed-secretref-name"

			errorList := ValidateBackupEntryUpdate(newBackupEntry, be)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow updating the name of the backup bucket", func() {
			newBackupEntry := prepareBackupEntryForUpdate(be)
			newBackupEntry.Spec.BucketName = "changed-bucket-name"

			errorList := ValidateBackupEntryUpdate(newBackupEntry, be)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow updating the region", func() {
			newBackupEntry := prepareBackupEntryForUpdate(be)
			newBackupEntry.Spec.Region = "changed-region"

			errorList := ValidateBackupEntryUpdate(newBackupEntry, be)

			Expect(errorList).To(BeEmpty())
		})
	})
})

func prepareBackupEntryForUpdate(obj *extensionsv1alpha1.BackupEntry) *extensionsv1alpha1.BackupEntry {
	newObj := obj.DeepCopy()
	newObj.ResourceVersion = "1"
	return newObj
}
