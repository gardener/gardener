// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclock "k8s.io/utils/clock/testing"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("BackupBucket", func() {
	var (
		clock     = testclock.NewFakeClock(metav1.Now().Time)
		condition = gardencorev1beta1.Condition{
			Type: "BackupBucketsReady",
		}
	)

	Describe("#ComputeBackupBucketsCondition", func() {
		It("should return Unknown when no BackupBuckets are provided", func() {
			c := ComputeBackupBucketsCondition(clock, condition, nil)
			Expect(c.Status).To(Equal(gardencorev1beta1.ConditionUnknown))
			Expect(c.Reason).To(Equal("BackupBucketsGone"))
		})

		It("should return True when all BackupBuckets are healthy", func() {
			bbs := []gardencorev1beta1.BackupBucket{
				{ObjectMeta: metav1.ObjectMeta{Name: "bb1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "bb2"}},
			}
			c := ComputeBackupBucketsCondition(clock, condition, bbs)
			Expect(c.Status).To(Equal(gardencorev1beta1.ConditionTrue))
			Expect(c.Reason).To(Equal("BackupBucketsAvailable"))
		})

		It("should return False when at least one BackupBucket has an error", func() {
			bbs := []gardencorev1beta1.BackupBucket{
				{ObjectMeta: metav1.ObjectMeta{Name: "bb1"}},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bb2"},
					Status:     gardencorev1beta1.BackupBucketStatus{LastError: &gardencorev1beta1.LastError{Description: "some error"}},
				},
			}
			c := ComputeBackupBucketsCondition(clock, condition, bbs)
			Expect(c.Status).To(Equal(gardencorev1beta1.ConditionFalse))
			Expect(c.Reason).To(Equal("BackupBucketsError"))
			Expect(c.Message).To(ContainSubstring("bb2"))
			Expect(c.Message).To(ContainSubstring("some error"))
		})
	})
})
