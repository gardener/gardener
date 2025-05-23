// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/validation"
)

var _ = Describe("validation", func() {
	var backupEntry *core.BackupEntry

	BeforeEach(func() {
		backupEntry = &core.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "example-backup-entry",
				Namespace: "garden",
				Annotations: map[string]string{
					core.BackupEntryForceDeletion: "true",
				},
			},
			Spec: core.BackupEntrySpec{
				BucketName: "some-bucket",
			},
		}
	})

	Describe("#ValidateBackupEntry", func() {
		It("should not return any errors", func() {
			errorList := ValidateBackupEntry(backupEntry)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid BackupEntry resources with empty metadata", func() {
			backupEntry.ObjectMeta = metav1.ObjectMeta{}

			errorList := ValidateBackupEntry(backupEntry)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.namespace"),
				}))))
		})

		It("should forbid BackupEntry specification with empty or invalid keys", func() {
			backupEntry.Spec.BucketName = ""

			errorList := ValidateBackupEntry(backupEntry)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.bucketName"),
			}))))
		})
	})

	Context("#ValidateBackupEntryUpdate", func() {
		It("update should not return error", func() {
			newBackupEntry := prepareBackupEntryForUpdate(backupEntry)
			newBackupEntry.Spec.BucketName = "another-bucketName"

			errorList := ValidateBackupEntryUpdate(newBackupEntry, backupEntry)

			Expect(errorList).To(BeEmpty())
		})
	})
})

func prepareBackupEntryForUpdate(obj *core.BackupEntry) *core.BackupEntry {
	newObj := obj.DeepCopy()
	newObj.ResourceVersion = "1"
	return newObj
}
