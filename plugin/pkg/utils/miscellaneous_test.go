// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package utils_test

import (
	"testing"

	gardenercore "github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/plugin/pkg/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/pointer"
)

func TestUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Admission Utils Suite")
}

var _ = Describe("Miscellaneous", func() {

	var (
		shoot1 = gardenercore.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot1",
				Namespace: "garden-pr1",
			},
			Spec: gardenercore.ShootSpec{
				SeedName: pointer.StringPtr("seed1"),
			},
		}

		shoot2 = gardenercore.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot2",
				Namespace: "garden-pr1",
			},
			Spec: gardenercore.ShootSpec{
				SeedName: pointer.StringPtr("seed1"),
			},
			Status: gardenercore.ShootStatus{
				SeedName: pointer.StringPtr("seed2"),
			},
		}

		shoot3 = gardenercore.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot3",
				Namespace: "garden-pr1",
			},
			Spec: gardenercore.ShootSpec{
				SeedName: nil,
			},
		}

		backupBucket1 = gardenercore.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bucket1",
			},
			Spec: gardenercore.BackupBucketSpec{
				SeedName: pointer.StringPtr("seed1"),
			},
		}
		backupBucket2 = gardenercore.BackupBucket{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bucket2",
			},
		}
	)
	backupBuckets := []*gardenercore.BackupBucket{
		&backupBucket1,
		&backupBucket2,
	}

	shoots := []*gardenercore.Shoot{
		&shoot1,
		&shoot2,
		&shoot3,
	}
	now := metav1.Now()

	DescribeTable("#SkipVerification",
		func(operation admission.Operation, metadata metav1.ObjectMeta, expected bool) {
			Expect(utils.SkipVerification(operation, metadata)).To(Equal(expected))
		},
		Entry("operation create with nil metadata", admission.Create, nil, false),
		Entry("operation connect with nil metadata", admission.Connect, nil, false),
		Entry("operation delete with nil metadata", admission.Delete, nil, false),
		Entry("operation create and object with deletion timestamp", admission.Create, metav1.ObjectMeta{DeletionTimestamp: &now}, false),
		Entry("operation update and object with deletion timestamp", admission.Update, metav1.ObjectMeta{DeletionTimestamp: &now}, true),
		Entry("operation update and object without deletion timestamp", admission.Update, metav1.ObjectMeta{Name: "obj1"}, false),
	)

	DescribeTable("#UsedByShoot",
		func(seedName string, expected bool) {
			Expect(utils.IsSeedUsedByShoot(seedName, shoots)).To(Equal(expected))
		},
		Entry("is used by shoot", "seed1", true),
		Entry("is used by shoot in migration", "seed2", true),
		Entry("is unused", "seed3", false),
	)

	DescribeTable("#UsedByBackupBucket",
		func(seedName string, expected bool) {
			Expect(utils.IsSeedUsedByBackupBucket(seedName, backupBuckets)).To(Equal(expected))
		},
		Entry("is used by backupbucket", "seed1", true),
		Entry("is not used by backupbucket", "seed2", false),
	)

})
