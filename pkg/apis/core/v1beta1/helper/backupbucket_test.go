// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

var _ = Describe("Helper", func() {
	DescribeTable("#BackupBucketIsErroneous",
		func(bb *gardencorev1beta1.BackupBucket, matcher1, matcher2 gomegatypes.GomegaMatcher) {
			erroneous, msg := BackupBucketIsErroneous(bb)
			Expect(erroneous).To(matcher1)
			Expect(msg).To(matcher2)
		},

		Entry("W/o BackupBucket", nil, BeFalse(), BeEmpty()),
		Entry("W/o last error", &gardencorev1beta1.BackupBucket{}, BeFalse(), BeEmpty()),
		Entry("W/ last error",
			&gardencorev1beta1.BackupBucket{Status: gardencorev1beta1.BackupBucketStatus{LastError: &gardencorev1beta1.LastError{Description: "foo"}}},
			BeTrue(),
			Equal("foo"),
		),
	)
})
