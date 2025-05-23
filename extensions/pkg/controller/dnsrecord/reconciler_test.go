// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsrecord

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

var _ = Describe("reconciler", func() {
	DescribeTable("#updateCreatedCondition", func(old v1beta1.ConditionStatus, f controller.UpdaterFunc, expected v1beta1.ConditionStatus) {
		var err error
		status := &extensionsv1alpha1.DefaultStatus{}
		switch old {
		case v1beta1.ConditionUnknown:
		case v1beta1.ConditionTrue:
			err = addCreatedConditionTrue(status)
			Expect(err).NotTo(HaveOccurred())
		case v1beta1.ConditionFalse:
			err = addCreatedConditionFalse(status)
			Expect(err).NotTo(HaveOccurred())
		}
		oldTime := time.Now().Add(-1 * time.Second)
		if len(status.Conditions) > 0 {
			status.Conditions[0].LastUpdateTime.Time = oldTime
			status.Conditions[0].LastUpdateTime.Time = oldTime
		}

		err = f(status)
		Expect(err).NotTo(HaveOccurred())
		Expect(status.Conditions).To(HaveLen(1))
		Expect(status.Conditions[0].Status).To(Equal(expected))
		if old != expected {
			Expect(status.Conditions[0].LastUpdateTime.Time).NotTo(Equal(oldTime))
		} else {
			Expect(status.Conditions[0].LastUpdateTime.Time).To(Equal(oldTime))
		}
	},

		Entry("empty -> success => true", v1beta1.ConditionUnknown, addCreatedConditionTrue, v1beta1.ConditionTrue),
		Entry("empty -> error => false", v1beta1.ConditionUnknown, addCreatedConditionFalse, v1beta1.ConditionFalse),
		Entry("success -> success => true", v1beta1.ConditionTrue, addCreatedConditionTrue, v1beta1.ConditionTrue),
		Entry("success -> error => true", v1beta1.ConditionTrue, addCreatedConditionFalse, v1beta1.ConditionTrue),
		Entry("error -> success => false", v1beta1.ConditionFalse, addCreatedConditionTrue, v1beta1.ConditionTrue),
		Entry("error -> error => false", v1beta1.ConditionFalse, addCreatedConditionFalse, v1beta1.ConditionFalse),
	)
})
