// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

			Expect(errorList).To(HaveLen(0))
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

			Expect(errorList).To(HaveLen(0))
		})
	})
})

func prepareBackupEntryForUpdate(obj *core.BackupEntry) *core.BackupEntry {
	newObj := obj.DeepCopy()
	newObj.ResourceVersion = "1"
	return newObj
}
