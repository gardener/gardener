// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package backupentry_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/gardenlet/controller/backupentry"
)

var _ = Describe("#IsBackupEntryManagedByThisGardenlet", func() {
	const (
		name      = "test"
		namespace = "garden"
		seedName  = "test-seed"
		otherSeed = "new-test-seed"
	)

	var backupEntry *gardencorev1beta1.BackupEntry

	BeforeEach(func() {
		backupEntry = &gardencorev1beta1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}
	})

	DescribeTable("check BackupEntry by seedName",
		func(bucketSeedName string, match gomegatypes.GomegaMatcher) {
			backupEntry.Spec.SeedName = pointer.String(bucketSeedName)
			gc := &config.GardenletConfiguration{
				SeedConfig: &config.SeedConfig{
					SeedTemplate: gardencore.SeedTemplate{
						ObjectMeta: metav1.ObjectMeta{
							Name: seedName,
						},
					},
				},
			}
			Expect(backupentry.IsBackupEntryManagedByThisGardenlet(backupEntry, gc)).To(match)
		},
		Entry("BackupEntry is not managed by this seed", otherSeed, BeFalse()),
		Entry("BackupEntry is managed by this seed", seedName, BeTrue()),
	)
})
