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

var _ = Describe("BackupBucket validation tests", func() {
	var bb *extensionsv1alpha1.BackupBucket

	BeforeEach(func() {
		bb = &extensionsv1alpha1.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-bb",
			},
			Spec: extensionsv1alpha1.BackupBucketSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: "provider",
				},
				Region: "region",
				SecretRef: corev1.SecretReference{
					Name: "test",
				},
			},
		}
	})

	Describe("#ValidBackupBucket", func() {
		It("should forbid empty BackupBucket resources", func() {
			errorList := ValidateBackupBucket(&extensionsv1alpha1.BackupBucket{})

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
				"Field": Equal("spec.secretRef.name"),
			}))))
		})

		It("should allow valid bb resources", func() {
			errorList := ValidateBackupBucket(bb)

			Expect(errorList).To(BeEmpty())
		})
	})

	Describe("#ValidBackupBucketUpdate", func() {
		It("should prevent updating anything if deletion time stamp is set", func() {
			now := metav1.Now()
			bb.DeletionTimestamp = &now

			newBackupBucket := prepareBackupBucketForUpdate(bb)
			newBackupBucket.DeletionTimestamp = &now
			newBackupBucket.Spec.SecretRef.Name = "changed-secretref-name"

			errorList := ValidateBackupBucketUpdate(newBackupBucket, bb)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec"),
				"Detail": Equal("cannot update backup bucket spec if deletion timestamp is set. Requested changes: SecretRef.Name: changed-secretref-name != test"),
			}))))
		})

		It("should prevent updating the type and region", func() {
			newBackupBucket := prepareBackupBucketForUpdate(bb)
			newBackupBucket.Spec.Type = "changed-type"
			newBackupBucket.Spec.Region = "changed-region"

			errorList := ValidateBackupBucketUpdate(newBackupBucket, bb)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.type"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.region"),
			}))))
		})

		It("should allow updating the name of the referenced secret", func() {
			newBackupBucket := prepareBackupBucketForUpdate(bb)
			newBackupBucket.Spec.SecretRef.Name = "changed-secretref-name"

			errorList := ValidateBackupBucketUpdate(newBackupBucket, bb)

			Expect(errorList).To(BeEmpty())
		})
	})
})

func prepareBackupBucketForUpdate(obj *extensionsv1alpha1.BackupBucket) *extensionsv1alpha1.BackupBucket {
	newObj := obj.DeepCopy()
	newObj.ResourceVersion = "1"
	return newObj
}
