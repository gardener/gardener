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
		controlPlaneNamespace = "shoot--foo--bar"
		shootStatusUID        = types.UID("5678")
		shootUID              = types.UID("1234")
		backupEntryName       = controlPlaneNamespace + "--" + string(shootStatusUID)
		sourceBackupEntryName = "source-" + controlPlaneNamespace + "--" + string(shootStatusUID)
	)

	Describe("#GenerateBackupEntryName", func() {
		It("should compute the correct name (using status UID)", func() {
			result, err := GenerateBackupEntryName(controlPlaneNamespace, shootStatusUID, shootUID)
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(backupEntryName))
		})

		It("should compute the correct name (using metadata UID)", func() {
			result, err := GenerateBackupEntryName(controlPlaneNamespace, "", shootUID)
			Expect(err).To(Not(HaveOccurred()))
			Expect(result).To(Equal(controlPlaneNamespace + "--" + string(shootUID)))
		})

		It("should fail if the shoot technical ID is empty", func() {
			_, err := GenerateBackupEntryName("", shootStatusUID, shootUID)
			Expect(err).To(HaveOccurred())
		})

		It("should fail if the shoot UID is empty", func() {
			_, err := GenerateBackupEntryName(controlPlaneNamespace, "", "")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#ExtractShootDetailsFromBackupEntryName", func() {
		It("should return the correct parts of the name for core backupentry", func() {
			namespace, uid := ExtractShootDetailsFromBackupEntryName(backupEntryName)
			Expect(namespace).To(Equal(controlPlaneNamespace))
			Expect(uid).To(Equal(shootStatusUID))
		})

		It("should return the correct parts of the name for source backupentry", func() {
			namespace, uid := ExtractShootDetailsFromBackupEntryName(sourceBackupEntryName)
			Expect(namespace).To(Equal(controlPlaneNamespace))
			Expect(uid).To(Equal(shootStatusUID))
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
