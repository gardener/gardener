// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/gardener/gardener/pkg/apis/core"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/gardener/gardener/pkg/apis/core/validation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
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
				SecretRef: corev1.SecretReference{
					Name:      "backup-secret",
					Namespace: "garden",
				},
				Seed: &seed,
			},
		}
	})

	Describe("#ValidateBackupBucket", func() {
		It("should not return any errors", func() {
			errorList := ValidateBackupBucket(backupBucket)

			Expect(errorList).To(HaveLen(0))
		})

		It("should forbid BackupBucket resources with empty metadata", func() {
			backupBucket.ObjectMeta = metav1.ObjectMeta{}

			errorList := ValidateBackupBucket(backupBucket)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			}))))
		})

		It("should forbid BackupBucket specification with empty or invalid keys", func() {
			backupBucket.Spec.Provider.Type = ""
			backupBucket.Spec.Provider.Region = ""
			backupBucket.Spec.SecretRef = corev1.SecretReference{}
			backupBucket.Spec.Seed = nil

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
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.secretRef.name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.secretRef.namespace"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.seed"),
				}))))
		})

		It("should forbid updating some keys", func() {
			newBackupBucket := prepareBackupBucketForUpdate(backupBucket)
			newBackupBucket.Spec.Provider.Type = "another-type"
			newBackupBucket.Spec.Provider.Region = "another-region"
			seed := "another-seed"
			newBackupBucket.Spec.Seed = &seed

			errorList := ValidateBackupBucketUpdate(newBackupBucket, backupBucket)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.provider"),
			})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.seed"),
				}))))
		})
	})

})

func prepareBackupBucketForUpdate(obj *core.BackupBucket) *core.BackupBucket {
	newObj := obj.DeepCopy()
	newObj.ResourceVersion = "1"
	return newObj
}
