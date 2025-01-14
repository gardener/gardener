// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

var _ = Describe("Helper", func() {
	DescribeTable("#HasOperationAnnotation",
		func(objectMeta metav1.ObjectMeta, expected bool) {
			Expect(HasOperationAnnotation(objectMeta.Annotations)).To(Equal(expected))
		},
		Entry("reconcile", metav1.ObjectMeta{Annotations: map[string]string{v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile}}, true),
		Entry("restore", metav1.ObjectMeta{Annotations: map[string]string{v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationRestore}}, true),
		Entry("migrate", metav1.ObjectMeta{Annotations: map[string]string{v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationMigrate}}, true),
		Entry("unknown", metav1.ObjectMeta{Annotations: map[string]string{v1beta1constants.GardenerOperation: "unknown"}}, false),
		Entry("not present", metav1.ObjectMeta{}, false),
	)

	DescribeTable("#AccessRestrictionsAreSupported",
		func(seedAccessRestrictions []gardencorev1beta1.AccessRestriction, shootAccessRestrictions []gardencorev1beta1.AccessRestrictionWithOptions, expectation bool) {
			Expect(AccessRestrictionsAreSupported(seedAccessRestrictions, shootAccessRestrictions)).To(Equal(expectation))
		},

		Entry("both have no access restrictions",
			nil,
			nil,
			true,
		),
		Entry("shoot has no access restrictions",
			[]gardencorev1beta1.AccessRestriction{{Name: "foo"}},
			nil,
			true,
		),
		Entry("seed has no access restrictions",
			nil,
			[]gardencorev1beta1.AccessRestrictionWithOptions{{AccessRestriction: gardencorev1beta1.AccessRestriction{Name: "foo"}}},
			false,
		),
		Entry("both have access restrictions and they match",
			[]gardencorev1beta1.AccessRestriction{{Name: "foo"}},
			[]gardencorev1beta1.AccessRestrictionWithOptions{{AccessRestriction: gardencorev1beta1.AccessRestriction{Name: "foo"}}},
			true,
		),
		Entry("both have access restrictions and they don't match",
			[]gardencorev1beta1.AccessRestriction{{Name: "bar"}},
			[]gardencorev1beta1.AccessRestrictionWithOptions{{AccessRestriction: gardencorev1beta1.AccessRestriction{Name: "foo"}}},
			false,
		),
	)

	Describe("Version helper", func() {

		DescribeTable("#UpsertLastError",
			func(lastErrors []gardencorev1beta1.LastError, lastError gardencorev1beta1.LastError, expected []gardencorev1beta1.LastError) {
				Expect(UpsertLastError(lastErrors, lastError)).To(Equal(expected))
			},

			Entry(
				"insert",
				[]gardencorev1beta1.LastError{
					{},
					{TaskID: ptr.To("bar")},
				},
				gardencorev1beta1.LastError{TaskID: ptr.To("foo"), Description: "error"},
				[]gardencorev1beta1.LastError{
					{},
					{TaskID: ptr.To("bar")},
					{TaskID: ptr.To("foo"), Description: "error"},
				},
			),
			Entry(
				"update",
				[]gardencorev1beta1.LastError{
					{},
					{TaskID: ptr.To("foo"), Description: "error"},
					{TaskID: ptr.To("bar")},
				},
				gardencorev1beta1.LastError{TaskID: ptr.To("foo"), Description: "new-error"},
				[]gardencorev1beta1.LastError{
					{},
					{TaskID: ptr.To("foo"), Description: "new-error"},
					{TaskID: ptr.To("bar")},
				},
			),
		)

		DescribeTable("#DeleteLastErrorByTaskID",
			func(lastErrors []gardencorev1beta1.LastError, taskID string, expected []gardencorev1beta1.LastError) {
				Expect(DeleteLastErrorByTaskID(lastErrors, taskID)).To(Equal(expected))
			},

			Entry(
				"task id not found",
				[]gardencorev1beta1.LastError{
					{},
					{TaskID: ptr.To("bar")},
				},
				"foo",
				[]gardencorev1beta1.LastError{
					{},
					{TaskID: ptr.To("bar")},
				},
			),
			Entry(
				"task id found",
				[]gardencorev1beta1.LastError{
					{},
					{TaskID: ptr.To("foo")},
					{TaskID: ptr.To("bar")},
				},
				"foo",
				[]gardencorev1beta1.LastError{
					{},
					{TaskID: ptr.To("bar")},
				},
			),
		)
	})

	DescribeTable("#IsFailureToleranceTypeZone",
		func(failureToleranceType *gardencorev1beta1.FailureToleranceType, expectedResult bool) {
			Expect(IsFailureToleranceTypeZone(failureToleranceType)).To(Equal(expectedResult))
		},

		Entry("failureToleranceType is zone", ptr.To(gardencorev1beta1.FailureToleranceTypeZone), true),
		Entry("failureToleranceType is node", ptr.To(gardencorev1beta1.FailureToleranceTypeNode), false),
		Entry("failureToleranceType is nil", nil, false),
	)

	DescribeTable("#IsFailureToleranceTypeNode",
		func(failureToleranceType *gardencorev1beta1.FailureToleranceType, expectedResult bool) {
			Expect(IsFailureToleranceTypeNode(failureToleranceType)).To(Equal(expectedResult))
		},

		Entry("failureToleranceType is zone", ptr.To(gardencorev1beta1.FailureToleranceTypeZone), false),
		Entry("failureToleranceType is node", ptr.To(gardencorev1beta1.FailureToleranceTypeNode), true),
		Entry("failureToleranceType is nil", nil, false),
	)

	DescribeTable("#ShootHasOperationType",
		func(lastOperation *gardencorev1beta1.LastOperation, lastOperationType gardencorev1beta1.LastOperationType, matcher gomegatypes.GomegaMatcher) {
			Expect(ShootHasOperationType(lastOperation, lastOperationType)).To(matcher)
		},
		Entry("last operation nil", nil, gardencorev1beta1.LastOperationTypeCreate, BeFalse()),
		Entry("last operation type does not match", &gardencorev1beta1.LastOperation{}, gardencorev1beta1.LastOperationTypeCreate, BeFalse()),
		Entry("last operation type matches", &gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeCreate}, gardencorev1beta1.LastOperationTypeCreate, BeTrue()),
	)
})
