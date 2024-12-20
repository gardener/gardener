// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("BackupEntry", func() {
	var (
		shootTechnicalID      = "seednamespace"
		shootUID              = types.UID("1234")
		backupEntryName       = shootTechnicalID + "--" + string(shootUID)
		sourceBackupEntryName = "source-" + shootTechnicalID + "--" + string(shootUID)
	)

	Describe("#GenerateBackupEntryName", func() {
		It("should compute the correct name", func() {
			result, err := GenerateBackupEntryName(shootTechnicalID, shootUID)
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(backupEntryName))
		})

		It("should fail if the shoot technical ID is empty", func() {
			_, err := GenerateBackupEntryName("", shootUID)
			Expect(err).To(HaveOccurred())
		})

		It("should fail if the shoot UID is empty", func() {
			_, err := GenerateBackupEntryName(shootTechnicalID, "")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#ExtractShootDetailsFromBackupEntryName", func() {
		It("should return the correct parts of the name for core backupentry", func() {
			technicalID, uid := ExtractShootDetailsFromBackupEntryName(backupEntryName)
			Expect(technicalID).To(Equal(shootTechnicalID))
			Expect(uid).To(Equal(shootUID))
		})

		It("should return the correct parts of the name for source backupentry", func() {
			technicalID, uid := ExtractShootDetailsFromBackupEntryName(sourceBackupEntryName)
			Expect(technicalID).To(Equal(shootTechnicalID))
			Expect(uid).To(Equal(shootUID))
		})
	})

	Describe("#GetBackupEntrySeedNames", func() {
		It("returns nil for other objects than BackupEntry", func() {
			specSeedName, statusSeedName := GetBackupEntrySeedNames(&corev1.Secret{})
			Expect(specSeedName).To(BeNil())
			Expect(statusSeedName).To(BeNil())
		})

		It("returns the correct seed names of a BackupEntry", func() {
			specSeedName, statusSeedName := GetBackupEntrySeedNames(&gardencorev1beta1.BackupEntry{
				Spec: gardencorev1beta1.BackupEntrySpec{
					SeedName: ptr.To("spec"),
				},
				Status: gardencorev1beta1.BackupEntryStatus{
					SeedName: ptr.To("status"),
				},
			})
			Expect(specSeedName).To(Equal(ptr.To("spec")))
			Expect(statusSeedName).To(Equal(ptr.To("status")))
		})
	})
})
