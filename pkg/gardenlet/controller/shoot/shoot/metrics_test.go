// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus/testutil"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardenletmetrics "github.com/gardener/gardener/pkg/gardenlet/metrics"
)

var _ = Describe("#reportMetrics", func() {
	var shoot *gardencorev1beta1.Shoot

	BeforeEach(func() {
		shoot = &gardencorev1beta1.Shoot{}
		gardenletmetrics.ShootOperationDurationSeconds.Reset()
	})

	DescribeTable("should record the duration",
		func(operationType gardencorev1beta1.LastOperationType) {
			reportMetrics(shoot, operationType, 5*time.Minute)
			Expect(testutil.CollectAndCount(gardenletmetrics.ShootOperationDurationSeconds)).To(Equal(1))
		},
		Entry("for Create", gardencorev1beta1.LastOperationTypeCreate),
		Entry("for Delete", gardencorev1beta1.LastOperationTypeDelete),
	)

	DescribeTable("should not record the duration",
		func(operationType gardencorev1beta1.LastOperationType) {
			reportMetrics(shoot, operationType, 30*24*time.Hour)
			Expect(testutil.CollectAndCount(gardenletmetrics.ShootOperationDurationSeconds)).To(Equal(0))
		},
		Entry("for Reconcile", gardencorev1beta1.LastOperationTypeReconcile),
		Entry("for Restore", gardencorev1beta1.LastOperationTypeRestore),
		Entry("for Migrate", gardencorev1beta1.LastOperationTypeMigrate),
	)
})
