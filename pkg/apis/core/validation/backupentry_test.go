// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	"k8s.io/utils/ptr"

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
				SeedName:   ptr.To("some-seed"),
			},
		}
	})

	Describe("#ValidateBackupEntry", func() {
		It("should not return any errors", func() {
			Expect(ValidateBackupEntry(backupEntry)).To(BeEmpty())
		})

		It("should not return any errors when shootRef is set", func() {
			backupEntry.Spec.SeedName = nil
			backupEntry.Spec.ShootRef = &corev1.ObjectReference{
				APIVersion: "core.gardener.cloud/v1beta1",
				Kind:       "Shoot",
				Name:       "shoot-name",
				Namespace:  backupEntry.Namespace,
			}

			Expect(ValidateBackupEntry(backupEntry)).To(BeEmpty())
		})

		It("should forbid referencing a Shoot from another namespace", func() {
			backupEntry.Spec.SeedName = nil
			backupEntry.Spec.ShootRef = &corev1.ObjectReference{
				APIVersion: "core.gardener.cloud/v1beta1",
				Kind:       "Shoot",
				Name:       "shoot-name",
				Namespace:  "shoot-namespace",
			}

			Expect(ValidateBackupEntry(backupEntry)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("spec.shootRef.namespace"),
			}))))
		})

		It("should forbid BackupEntry resources with empty metadata", func() {
			backupEntry.ObjectMeta = metav1.ObjectMeta{}

			Expect(ValidateBackupEntry(backupEntry)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.namespace"),
				})),
			))
		})

		It("should forbid BackupEntry specification with empty or invalid keys", func() {
			backupEntry.Spec.BucketName = ""

			Expect(ValidateBackupEntry(backupEntry)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.bucketName"),
			}))))
		})

		It("should forbid specifying neither seedName nor shootRef", func() {
			backupEntry.Spec.SeedName = nil

			Expect(ValidateBackupEntry(backupEntry)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.seedName"),
			}))))
		})

		It("should forbid specifying both seedName and shootRef", func() {
			backupEntry.Spec.ShootRef = &corev1.ObjectReference{
				APIVersion: "core.gardener.cloud/v1beta1",
				Kind:       "Shoot",
				Name:       "shoot-name",
				Namespace:  "shoot-namespace",
			}

			Expect(ValidateBackupEntry(backupEntry)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.seedName"),
			}))))
		})

		It("should forbid specifying an empty seed name", func() {
			backupEntry.Spec.SeedName = ptr.To("")

			Expect(ValidateBackupEntry(backupEntry)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.seedName"),
			}))))
		})

		It("should forbid specifying an invalid shoot ref", func() {
			backupEntry.Spec.SeedName = nil
			backupEntry.Spec.ShootRef = &corev1.ObjectReference{}

			Expect(ValidateBackupEntry(backupEntry)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.shootRef.apiVersion"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.shootRef.kind"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.shootRef.name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.shootRef.namespace"),
				})),
			))
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
