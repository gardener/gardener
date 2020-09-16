// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerregistration

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Controller", func() {
	var (
		queue *fakeQueue
		c     *Controller

		seedName = "seed"
	)

	BeforeEach(func() {
		queue = &fakeQueue{}
		c = &Controller{
			controllerRegistrationSeedQueue: queue,
		}
	})

	Describe("#backupEntryAdd", func() {
		It("should do nothing because object is not a BackupEntry", func() {
			obj := &gardencorev1beta1.CloudProfile{}

			c.backupEntryAdd(obj)

			Expect(queue.Len()).To(BeZero())
		})

		It("should do nothing because the seedName is nil", func() {
			obj := &gardencorev1beta1.BackupEntry{}

			c.backupEntryAdd(obj)

			Expect(queue.Len()).To(BeZero())
		})

		It("should add the object to the queue", func() {
			obj := &gardencorev1beta1.BackupEntry{
				Spec: gardencorev1beta1.BackupEntrySpec{
					SeedName: &seedName,
				},
			}

			c.backupEntryAdd(obj)

			Expect(queue.Len()).To(Equal(1))
			Expect(queue.items[0]).To(Equal(seedName))
		})
	})

	Describe("#backupEntryUpdate", func() {
		It("should do nothing because old object is not a BackupEntry", func() {
			oldObj := &gardencorev1beta1.CloudProfile{}
			newObj := &gardencorev1beta1.BackupEntry{}

			c.backupEntryUpdate(oldObj, newObj)

			Expect(queue.Len()).To(BeZero())
		})

		It("should do nothing because new object is not a BackupEntry", func() {
			oldObj := &gardencorev1beta1.BackupEntry{}
			newObj := &gardencorev1beta1.CloudProfile{}

			c.backupEntryUpdate(oldObj, newObj)

			Expect(queue.Len()).To(BeZero())
		})

		It("should do nothing because nothing changed", func() {
			oldObj := &gardencorev1beta1.BackupEntry{}
			newObj := &gardencorev1beta1.BackupEntry{}

			c.backupEntryUpdate(oldObj, newObj)

			Expect(queue.Len()).To(BeZero())
		})

		It("should add the new obj to the queue because seed name changed", func() {
			oldObj := &gardencorev1beta1.BackupEntry{}
			newObj := &gardencorev1beta1.BackupEntry{
				Spec: gardencorev1beta1.BackupEntrySpec{
					SeedName: &seedName,
				},
			}

			c.backupEntryUpdate(oldObj, newObj)

			Expect(queue.Len()).To(Equal(1))
			Expect(queue.items[0]).To(Equal(seedName))
		})

		It("should add the new obj to the queue because bucket name changed", func() {
			oldObj := &gardencorev1beta1.BackupEntry{
				Spec: gardencorev1beta1.BackupEntrySpec{
					SeedName: &seedName,
				},
			}
			newObj := oldObj.DeepCopy()
			newObj.Spec.BucketName = "bucket"

			c.backupEntryUpdate(oldObj, newObj)

			Expect(queue.Len()).To(Equal(1))
			Expect(queue.items[0]).To(Equal(seedName))
		})
	})

	Describe("#backupEntryDelete", func() {
		It("should do nothing because object is not a BackupEntry", func() {
			obj := &gardencorev1beta1.CloudProfile{}

			c.backupEntryDelete(obj)

			Expect(queue.Len()).To(BeZero())
		})

		It("should do nothing because the seedName is nil", func() {
			obj := &gardencorev1beta1.BackupEntry{}

			c.backupEntryDelete(obj)

			Expect(queue.Len()).To(BeZero())
		})

		It("should add the object to the queue", func() {
			obj := &gardencorev1beta1.BackupEntry{
				Spec: gardencorev1beta1.BackupEntrySpec{
					SeedName: &seedName,
				},
			}

			c.backupEntryDelete(obj)

			Expect(queue.Len()).To(Equal(1))
			Expect(queue.items[0]).To(Equal(seedName))
		})
	})
})
