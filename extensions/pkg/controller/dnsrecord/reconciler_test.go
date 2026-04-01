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

	Describe("#getCreatedConditionStatus", func() {
		It("should return Unknown when there are no conditions", func() {
			status := &extensionsv1alpha1.DefaultStatus{}
			Expect(getCreatedConditionStatus(status)).To(Equal(v1beta1.ConditionUnknown))
		})

		It("should return Unknown when there are conditions but none of type Created", func() {
			status := &extensionsv1alpha1.DefaultStatus{
				Conditions: []v1beta1.Condition{
					{Type: "SomeOtherCondition", Status: v1beta1.ConditionTrue},
				},
			}
			Expect(getCreatedConditionStatus(status)).To(Equal(v1beta1.ConditionUnknown))
		})

		It("should return True when the Created condition has status True", func() {
			status := &extensionsv1alpha1.DefaultStatus{}
			Expect(addCreatedConditionTrue(status)).To(Succeed())
			Expect(getCreatedConditionStatus(status)).To(Equal(v1beta1.ConditionTrue))
		})

		It("should return False when the Created condition has status False", func() {
			status := &extensionsv1alpha1.DefaultStatus{}
			Expect(addCreatedConditionFalse(status)).To(Succeed())
			Expect(getCreatedConditionStatus(status)).To(Equal(v1beta1.ConditionFalse))
		})

		It("should find the Created condition among multiple conditions", func() {
			status := &extensionsv1alpha1.DefaultStatus{
				Conditions: []v1beta1.Condition{
					{Type: "SomeCondition", Status: v1beta1.ConditionFalse},
					{Type: extensionsv1alpha1.ConditionTypeCreated, Status: v1beta1.ConditionTrue},
					{Type: "AnotherCondition", Status: v1beta1.ConditionUnknown},
				},
			}
			Expect(getCreatedConditionStatus(status)).To(Equal(v1beta1.ConditionTrue))
		})
	})

	Describe("#addCreatedConditionTrue", func() {
		It("should set the Created condition to True on empty status", func() {
			status := &extensionsv1alpha1.DefaultStatus{}
			Expect(addCreatedConditionTrue(status)).To(Succeed())

			Expect(status.Conditions).To(HaveLen(1))
			Expect(status.Conditions[0].Type).To(Equal(v1beta1.ConditionType(extensionsv1alpha1.ConditionTypeCreated)))
			Expect(status.Conditions[0].Status).To(Equal(v1beta1.ConditionTrue))
			Expect(status.Conditions[0].Reason).To(Equal("Success"))
			Expect(status.Conditions[0].Message).To(Equal("Record was created successfully in infrastructure at least once"))
		})

		It("should update the Created condition from False to True", func() {
			status := &extensionsv1alpha1.DefaultStatus{}
			Expect(addCreatedConditionFalse(status)).To(Succeed())
			Expect(status.Conditions[0].Status).To(Equal(v1beta1.ConditionFalse))

			Expect(addCreatedConditionTrue(status)).To(Succeed())
			Expect(status.Conditions).To(HaveLen(1))
			Expect(status.Conditions[0].Status).To(Equal(v1beta1.ConditionTrue))
		})

		It("should not change the condition when already True", func() {
			status := &extensionsv1alpha1.DefaultStatus{}
			Expect(addCreatedConditionTrue(status)).To(Succeed())
			oldTime := time.Now().Add(-1 * time.Second)
			status.Conditions[0].LastUpdateTime.Time = oldTime

			Expect(addCreatedConditionTrue(status)).To(Succeed())
			Expect(status.Conditions).To(HaveLen(1))
			Expect(status.Conditions[0].Status).To(Equal(v1beta1.ConditionTrue))
			Expect(status.Conditions[0].LastUpdateTime.Time).To(Equal(oldTime))
		})
	})

	Describe("#addCreatedConditionFalse", func() {
		It("should set the Created condition to False on empty status", func() {
			status := &extensionsv1alpha1.DefaultStatus{}
			Expect(addCreatedConditionFalse(status)).To(Succeed())

			Expect(status.Conditions).To(HaveLen(1))
			Expect(status.Conditions[0].Type).To(Equal(v1beta1.ConditionType(extensionsv1alpha1.ConditionTypeCreated)))
			Expect(status.Conditions[0].Status).To(Equal(v1beta1.ConditionFalse))
			Expect(status.Conditions[0].Reason).To(Equal("Error"))
			Expect(status.Conditions[0].Message).To(Equal("Error on initial record creation in infrastructure"))
		})

		It("should not update the Created condition from True to False (updateIfExisting=false)", func() {
			status := &extensionsv1alpha1.DefaultStatus{}
			Expect(addCreatedConditionTrue(status)).To(Succeed())
			Expect(status.Conditions[0].Status).To(Equal(v1beta1.ConditionTrue))

			Expect(addCreatedConditionFalse(status)).To(Succeed())
			Expect(status.Conditions).To(HaveLen(1))
			Expect(status.Conditions[0].Status).To(Equal(v1beta1.ConditionTrue))
		})

		It("should not change the condition when already False", func() {
			status := &extensionsv1alpha1.DefaultStatus{}
			Expect(addCreatedConditionFalse(status)).To(Succeed())
			oldTime := time.Now().Add(-1 * time.Second)
			status.Conditions[0].LastUpdateTime.Time = oldTime

			Expect(addCreatedConditionFalse(status)).To(Succeed())
			Expect(status.Conditions).To(HaveLen(1))
			Expect(status.Conditions[0].Status).To(Equal(v1beta1.ConditionFalse))
			Expect(status.Conditions[0].LastUpdateTime.Time).To(Equal(oldTime))
		})
	})
})
