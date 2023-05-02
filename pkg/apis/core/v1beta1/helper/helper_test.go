// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package helper_test

import (
	"fmt"
	"time"

	"github.com/Masterminds/semver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
)

var _ = Describe("helper", func() {
	var (
		trueVar                 = true
		falseVar                = false
		expirationDateInThePast = metav1.Time{Time: time.Now().AddDate(0, 0, -1)}
		fakeClock               = testclock.NewFakeClock(time.Now())
	)

	Describe("errors", func() {
		var (
			testTime      = metav1.NewTime(time.Unix(10, 10))
			zeroTime      metav1.Time
			afterTestTime = func(t metav1.Time) bool { return t.After(testTime.Time) }
		)

		DescribeTable("#UpdatedConditionWithClock",
			func(condition gardencorev1beta1.Condition, status gardencorev1beta1.ConditionStatus, reason, message string, codes []gardencorev1beta1.ErrorCode, matcher gomegatypes.GomegaMatcher) {
				updated := UpdatedConditionWithClock(fakeClock, condition, status, reason, message, codes...)

				Expect(updated).To(matcher)
			},
			Entry("initialize empty timestamps",
				gardencorev1beta1.Condition{
					Type:    "type",
					Status:  gardencorev1beta1.ConditionTrue,
					Reason:  "reason",
					Message: "message",
				},
				gardencorev1beta1.ConditionTrue,
				"reason",
				"message",
				nil,
				MatchFields(IgnoreExtras, Fields{
					"Status":             Equal(gardencorev1beta1.ConditionTrue),
					"Reason":             Equal("reason"),
					"Message":            Equal("message"),
					"LastTransitionTime": Not(Equal(zeroTime)),
					"LastUpdateTime":     Not(Equal(zeroTime)),
				}),
			),
			Entry("no update",
				gardencorev1beta1.Condition{
					Type:               "type",
					Status:             gardencorev1beta1.ConditionTrue,
					Reason:             "reason",
					Message:            "message",
					LastTransitionTime: testTime,
					LastUpdateTime:     testTime,
				},
				gardencorev1beta1.ConditionTrue,
				"reason",
				"message",
				nil,
				MatchFields(IgnoreExtras, Fields{
					"Status":             Equal(gardencorev1beta1.ConditionTrue),
					"Reason":             Equal("reason"),
					"Message":            Equal("message"),
					"LastTransitionTime": Equal(testTime),
					"LastUpdateTime":     Equal(testTime),
				}),
			),
			Entry("update reason",
				gardencorev1beta1.Condition{
					Type:               "type",
					Status:             gardencorev1beta1.ConditionTrue,
					Reason:             "reason",
					Message:            "message",
					LastTransitionTime: testTime,
					LastUpdateTime:     testTime,
				},
				gardencorev1beta1.ConditionTrue,
				"OtherReason",
				"message",
				nil,
				MatchFields(IgnoreExtras, Fields{
					"Status":             Equal(gardencorev1beta1.ConditionTrue),
					"Reason":             Equal("OtherReason"),
					"Message":            Equal("message"),
					"LastTransitionTime": Equal(testTime),
					"LastUpdateTime":     Satisfy(afterTestTime),
				}),
			),
			Entry("update codes",
				gardencorev1beta1.Condition{
					Type:               "type",
					Status:             gardencorev1beta1.ConditionTrue,
					Reason:             "reason",
					Message:            "message",
					LastTransitionTime: testTime,
					LastUpdateTime:     testTime,
				},
				gardencorev1beta1.ConditionTrue,
				"reason",
				"message",
				[]gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraQuotaExceeded},
				MatchFields(IgnoreExtras, Fields{
					"Status":             Equal(gardencorev1beta1.ConditionTrue),
					"Reason":             Equal("reason"),
					"Message":            Equal("message"),
					"Codes":              Equal([]gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraQuotaExceeded}),
					"LastTransitionTime": Equal(testTime),
					"LastUpdateTime":     Satisfy(afterTestTime),
				}),
			),
			Entry("update status",
				gardencorev1beta1.Condition{
					Type:               "type",
					Status:             gardencorev1beta1.ConditionTrue,
					Reason:             "reason",
					Message:            "message",
					LastTransitionTime: testTime,
					LastUpdateTime:     testTime,
				},
				gardencorev1beta1.ConditionFalse,
				"reason",
				"message",
				nil,
				MatchFields(IgnoreExtras, Fields{
					"Status":             Equal(gardencorev1beta1.ConditionFalse),
					"Reason":             Equal("reason"),
					"Message":            Equal("message"),
					"LastTransitionTime": Satisfy(afterTestTime),
					"LastUpdateTime":     Equal(testTime),
				}),
			),
			Entry("clear codes",
				gardencorev1beta1.Condition{
					Type:               "type",
					Status:             gardencorev1beta1.ConditionTrue,
					Reason:             "reason",
					Message:            "message",
					LastTransitionTime: testTime,
					LastUpdateTime:     testTime,
					Codes:              []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraQuotaExceeded},
				},
				gardencorev1beta1.ConditionTrue,
				"reason",
				"message",
				nil,
				MatchFields(IgnoreExtras, Fields{
					"Status":             Equal(gardencorev1beta1.ConditionTrue),
					"Reason":             Equal("reason"),
					"Message":            Equal("message"),
					"LastTransitionTime": Equal(testTime),
					"LastUpdateTime":     Satisfy(afterTestTime),
					"Codes":              BeEmpty(),
				}),
			),
		)

		Describe("#MergeConditions", func() {
			It("should merge the conditions", func() {
				var (
					typeFoo gardencorev1beta1.ConditionType = "foo"
					typeBar gardencorev1beta1.ConditionType = "bar"
				)

				oldConditions := []gardencorev1beta1.Condition{
					{
						Type:   typeFoo,
						Reason: "hugo",
					},
				}

				result := MergeConditions(oldConditions, gardencorev1beta1.Condition{Type: typeFoo}, gardencorev1beta1.Condition{Type: typeBar})

				Expect(result).To(Equal([]gardencorev1beta1.Condition{{Type: typeFoo}, {Type: typeBar}}))
			})
		})

		DescribeTable("#RemoveConditions",
			func(conditions []gardencorev1beta1.Condition, conditionTypes []gardencorev1beta1.ConditionType, expectedResult []gardencorev1beta1.Condition) {
				Expect(RemoveConditions(conditions, conditionTypes...)).To(Equal(expectedResult))
			},
			Entry("remove foo", []gardencorev1beta1.Condition{{Type: "foo"}, {Type: "bar"}}, []gardencorev1beta1.ConditionType{"foo"},
				[]gardencorev1beta1.Condition{{Type: "bar"}}),
			Entry("remove bar", []gardencorev1beta1.Condition{{Type: "foo"}, {Type: "bar"}}, []gardencorev1beta1.ConditionType{"bar"},
				[]gardencorev1beta1.Condition{{Type: "foo"}}),
			Entry("don't remove anything", []gardencorev1beta1.Condition{{Type: "foo"}, {Type: "bar"}}, nil,
				[]gardencorev1beta1.Condition{{Type: "foo"}, {Type: "bar"}}),
			Entry("remove from an empty slice", nil, []gardencorev1beta1.ConditionType{"foo"}, nil),
		)

		Describe("#GetCondition", func() {
			It("should return the found condition", func() {
				var (
					conditionType gardencorev1beta1.ConditionType = "test-1"
					condition                                     = gardencorev1beta1.Condition{
						Type: conditionType,
					}
					conditions = []gardencorev1beta1.Condition{condition}
				)

				cond := GetCondition(conditions, conditionType)

				Expect(cond).NotTo(BeNil())
				Expect(*cond).To(Equal(condition))
			})

			It("should return nil because the required condition could not be found", func() {
				var (
					conditionType gardencorev1beta1.ConditionType = "test-1"
					conditions                                    = []gardencorev1beta1.Condition{}
				)

				cond := GetCondition(conditions, conditionType)

				Expect(cond).To(BeNil())
			})
		})

		Describe("#GetOrInitConditionWithClock", func() {
			It("should get the existing condition", func() {
				var (
					c          = gardencorev1beta1.Condition{Type: "foo"}
					conditions = []gardencorev1beta1.Condition{c}
				)

				Expect(GetOrInitConditionWithClock(fakeClock, conditions, "foo")).To(Equal(c))
			})

			It("should return a new, initialized condition", func() {
				Expect(GetOrInitConditionWithClock(fakeClock, nil, "foo")).To(Equal(InitConditionWithClock(fakeClock, "foo")))
			})
		})

		DescribeTable("#IsResourceSupported",
			func(resources []gardencorev1beta1.ControllerResource, resourceKind, resourceType string, expectation bool) {
				Expect(IsResourceSupported(resources, resourceKind, resourceType)).To(Equal(expectation))
			},
			Entry("expect true",
				[]gardencorev1beta1.ControllerResource{
					{
						Kind: "foo",
						Type: "bar",
					},
				},
				"foo",
				"bar",
				true,
			),
			Entry("expect true",
				[]gardencorev1beta1.ControllerResource{
					{
						Kind: "foo",
						Type: "bar",
					},
				},
				"foo",
				"BAR",
				true,
			),
			Entry("expect false",
				[]gardencorev1beta1.ControllerResource{
					{
						Kind: "foo",
						Type: "bar",
					},
				},
				"foo",
				"baz",
				false,
			),
		)

		DescribeTable("#IsControllerInstallationSuccessful",
			func(conditions []gardencorev1beta1.Condition, expectation bool) {
				controllerInstallation := gardencorev1beta1.ControllerInstallation{
					Status: gardencorev1beta1.ControllerInstallationStatus{
						Conditions: conditions,
					},
				}
				Expect(IsControllerInstallationSuccessful(controllerInstallation)).To(Equal(expectation))
			},
			Entry("expect true",
				[]gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.ControllerInstallationInstalled,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationHealthy,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationProgressing,
						Status: gardencorev1beta1.ConditionFalse,
					},
				},
				true,
			),
			Entry("expect false",
				[]gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.ControllerInstallationInstalled,
						Status: gardencorev1beta1.ConditionFalse,
					},
				},
				false,
			),
			Entry("expect false",
				[]gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.ControllerInstallationHealthy,
						Status: gardencorev1beta1.ConditionFalse,
					},
				},
				false,
			),
			Entry("expect false",
				[]gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.ControllerInstallationProgressing,
						Status: gardencorev1beta1.ConditionTrue,
					},
				},
				false,
			),
			Entry("expect false",
				[]gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.ControllerInstallationInstalled,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationHealthy,
						Status: gardencorev1beta1.ConditionFalse,
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationProgressing,
						Status: gardencorev1beta1.ConditionFalse,
					},
				},
				false,
			),
			Entry("expect false",
				[]gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.ControllerInstallationInstalled,
						Status: gardencorev1beta1.ConditionFalse,
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationHealthy,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationProgressing,
						Status: gardencorev1beta1.ConditionFalse,
					},
				},
				false,
			),
			Entry("expect false",
				[]gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.ControllerInstallationInstalled,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationHealthy,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationProgressing,
						Status: gardencorev1beta1.ConditionTrue,
					},
				},
				false,
			),
			Entry("expect false",
				[]gardencorev1beta1.Condition{},
				false,
			),
		)

		DescribeTable("#IsControllerInstallationRequired",
			func(conditions []gardencorev1beta1.Condition, expectation bool) {
				controllerInstallation := gardencorev1beta1.ControllerInstallation{
					Status: gardencorev1beta1.ControllerInstallationStatus{
						Conditions: conditions,
					},
				}
				Expect(IsControllerInstallationRequired(controllerInstallation)).To(Equal(expectation))
			},
			Entry("expect true",
				[]gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.ControllerInstallationRequired,
						Status: gardencorev1beta1.ConditionTrue,
					},
				},
				true,
			),
			Entry("expect false",
				[]gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.ControllerInstallationRequired,
						Status: gardencorev1beta1.ConditionFalse,
					},
				},
				false,
			),
			Entry("expect false",
				[]gardencorev1beta1.Condition{},
				false,
			),
		)

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

		DescribeTable("#FindMachineTypeByName",
			func(machines []gardencorev1beta1.MachineType, name string, expectedMachine *gardencorev1beta1.MachineType) {
				Expect(FindMachineTypeByName(machines, name)).To(Equal(expectedMachine))
			},

			Entry("no workers", nil, "", nil),
			Entry("worker not found", []gardencorev1beta1.MachineType{{Name: "foo"}}, "bar", nil),
			Entry("worker found", []gardencorev1beta1.MachineType{{Name: "foo"}}, "foo", &gardencorev1beta1.MachineType{Name: "foo"}),
		)

		DescribeTable("#TaintsHave",
			func(taints []gardencorev1beta1.SeedTaint, key string, expectation bool) {
				Expect(TaintsHave(taints, key)).To(Equal(expectation))
			},
			Entry("taint exists", []gardencorev1beta1.SeedTaint{{Key: "foo"}}, "foo", true),
			Entry("taint does not exist", []gardencorev1beta1.SeedTaint{{Key: "foo"}}, "bar", false),
		)

		DescribeTable("#TaintsAreTolerated",
			func(taints []gardencorev1beta1.SeedTaint, tolerations []gardencorev1beta1.Toleration, expectation bool) {
				Expect(TaintsAreTolerated(taints, tolerations)).To(Equal(expectation))
			},

			Entry("no taints",
				nil,
				[]gardencorev1beta1.Toleration{{Key: "foo"}},
				true,
			),
			Entry("no tolerations",
				[]gardencorev1beta1.SeedTaint{{Key: "foo"}},
				nil,
				false,
			),
			Entry("taints with keys only, tolerations with keys only (tolerated)",
				[]gardencorev1beta1.SeedTaint{{Key: "foo"}},
				[]gardencorev1beta1.Toleration{{Key: "foo"}},
				true,
			),
			Entry("taints with keys only, tolerations with keys only (non-tolerated)",
				[]gardencorev1beta1.SeedTaint{{Key: "foo"}},
				[]gardencorev1beta1.Toleration{{Key: "bar"}},
				false,
			),
			Entry("taints with keys+values only, tolerations with keys+values only (tolerated)",
				[]gardencorev1beta1.SeedTaint{{Key: "foo", Value: pointer.String("bar")}},
				[]gardencorev1beta1.Toleration{{Key: "foo", Value: pointer.String("bar")}},
				true,
			),
			Entry("taints with keys+values only, tolerations with keys+values only (non-tolerated)",
				[]gardencorev1beta1.SeedTaint{{Key: "foo", Value: pointer.String("bar")}},
				[]gardencorev1beta1.Toleration{{Key: "bar", Value: pointer.String("foo")}},
				false,
			),
			Entry("taints with mixed key(+values), tolerations with mixed key(+values) (tolerated)",
				[]gardencorev1beta1.SeedTaint{
					{Key: "foo"},
					{Key: "bar", Value: pointer.String("baz")},
				},
				[]gardencorev1beta1.Toleration{
					{Key: "foo"},
					{Key: "bar", Value: pointer.String("baz")},
				},
				true,
			),
			Entry("taints with mixed key(+values), tolerations with mixed key(+values) (non-tolerated)",
				[]gardencorev1beta1.SeedTaint{
					{Key: "foo"},
					{Key: "bar", Value: pointer.String("baz")},
				},
				[]gardencorev1beta1.Toleration{
					{Key: "bar"},
					{Key: "foo", Value: pointer.String("baz")},
				},
				false,
			),
			Entry("taints with mixed key(+values), tolerations with key+values only (tolerated)",
				[]gardencorev1beta1.SeedTaint{
					{Key: "foo"},
					{Key: "bar", Value: pointer.String("baz")},
				},
				[]gardencorev1beta1.Toleration{
					{Key: "foo", Value: pointer.String("bar")},
					{Key: "bar", Value: pointer.String("baz")},
				},
				true,
			),
			Entry("taints with mixed key(+values), tolerations with key+values only (untolerated)",
				[]gardencorev1beta1.SeedTaint{
					{Key: "foo"},
					{Key: "bar", Value: pointer.String("baz")},
				},
				[]gardencorev1beta1.Toleration{
					{Key: "foo", Value: pointer.String("bar")},
					{Key: "bar", Value: pointer.String("foo")},
				},
				false,
			),
			Entry("taints > tolerations",
				[]gardencorev1beta1.SeedTaint{
					{Key: "foo"},
					{Key: "bar", Value: pointer.String("baz")},
				},
				[]gardencorev1beta1.Toleration{
					{Key: "bar", Value: pointer.String("baz")},
				},
				false,
			),
			Entry("tolerations > taints",
				[]gardencorev1beta1.SeedTaint{
					{Key: "foo"},
					{Key: "bar", Value: pointer.String("baz")},
				},
				[]gardencorev1beta1.Toleration{
					{Key: "foo", Value: pointer.String("bar")},
					{Key: "bar", Value: pointer.String("baz")},
					{Key: "baz", Value: pointer.String("foo")},
				},
				true,
			),
		)
	})

	Describe("#ReadManagedSeedAPIServer", func() {
		var shoot *gardencorev1beta1.Shoot

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   v1beta1constants.GardenNamespace,
					Annotations: nil,
				},
			}
		})

		It("should return nil,nil when the Shoot is not in the garden namespace", func() {
			shoot.Namespace = "garden-dev"

			settings, err := ReadManagedSeedAPIServer(shoot)

			Expect(err).NotTo(HaveOccurred())
			Expect(settings).To(BeNil())
		})

		It("should return nil,nil when the annotations are nil", func() {
			settings, err := ReadManagedSeedAPIServer(shoot)

			Expect(err).NotTo(HaveOccurred())
			Expect(settings).To(BeNil())
		})

		It("should return nil,nil when the annotation is not set", func() {
			shoot.Annotations = map[string]string{
				"foo": "bar",
			}

			settings, err := ReadManagedSeedAPIServer(shoot)

			Expect(err).NotTo(HaveOccurred())
			Expect(settings).To(BeNil())
		})

		It("should return err when minReplicas is specified but maxReplicas is not", func() {
			shoot.Annotations = map[string]string{
				v1beta1constants.AnnotationManagedSeedAPIServer: "apiServer.autoscaler.minReplicas=3",
			}

			settings, err := ReadManagedSeedAPIServer(shoot)

			Expect(err).To(MatchError("apiSrvMaxReplicas has to be specified for ManagedSeed API server autoscaler"))
			Expect(settings).To(BeNil())
		})

		It("should return err when minReplicas fails to be parsed", func() {
			shoot.Annotations = map[string]string{
				v1beta1constants.AnnotationManagedSeedAPIServer: "apiServer.autoscaler.minReplicas=foo,,apiServer.autoscaler.maxReplicas=3",
			}

			settings, err := ReadManagedSeedAPIServer(shoot)

			Expect(err).To(HaveOccurred())
			Expect(settings).To(BeNil())
		})

		It("should return err when maxReplicas fails to be parsed", func() {
			shoot.Annotations = map[string]string{
				v1beta1constants.AnnotationManagedSeedAPIServer: "apiServer.autoscaler.minReplicas=3,apiServer.autoscaler.maxReplicas=foo",
			}

			settings, err := ReadManagedSeedAPIServer(shoot)

			Expect(err).To(HaveOccurred())
			Expect(settings).To(BeNil())
		})

		It("should return err when replicas fails to be parsed", func() {
			shoot.Annotations = map[string]string{
				v1beta1constants.AnnotationManagedSeedAPIServer: "apiServer.replicas=foo,apiServer.autoscaler.minReplicas=3,apiServer.autoscaler.maxReplicas=3",
			}

			settings, err := ReadManagedSeedAPIServer(shoot)

			Expect(err).To(HaveOccurred())
			Expect(settings).To(BeNil())
		})

		It("should return err when replicas is invalid", func() {
			shoot.Annotations = map[string]string{
				v1beta1constants.AnnotationManagedSeedAPIServer: "apiServer.replicas=-1,apiServer.autoscaler.minReplicas=3,apiServer.autoscaler.maxReplicas=3",
			}

			settings, err := ReadManagedSeedAPIServer(shoot)

			Expect(err).To(HaveOccurred())
			Expect(settings).To(BeNil())
		})

		It("should return err when minReplicas is greater than maxReplicas", func() {
			shoot.Annotations = map[string]string{
				v1beta1constants.AnnotationManagedSeedAPIServer: "apiServer.replicas=3,apiServer.autoscaler.minReplicas=3,apiServer.autoscaler.maxReplicas=2",
			}

			settings, err := ReadManagedSeedAPIServer(shoot)

			Expect(err).To(HaveOccurred())
			Expect(settings).To(BeNil())
		})

		It("should return the default the minReplicas and maxReplicas settings when they are not provided", func() {
			shoot.Annotations = map[string]string{
				v1beta1constants.AnnotationManagedSeedAPIServer: "apiServer.replicas=3",
			}

			settings, err := ReadManagedSeedAPIServer(shoot)

			Expect(err).NotTo(HaveOccurred())
			Expect(settings).To(Equal(&ManagedSeedAPIServer{
				Replicas: pointer.Int32(3),
				Autoscaler: &ManagedSeedAPIServerAutoscaler{
					MinReplicas: pointer.Int32(3),
					MaxReplicas: 3,
				},
			}))
		})

		It("should return the configured settings", func() {
			shoot.Annotations = map[string]string{
				v1beta1constants.AnnotationManagedSeedAPIServer: "apiServer.replicas=3,apiServer.autoscaler.minReplicas=3,apiServer.autoscaler.maxReplicas=6",
			}

			settings, err := ReadManagedSeedAPIServer(shoot)

			Expect(err).NotTo(HaveOccurred())
			Expect(settings).To(Equal(&ManagedSeedAPIServer{
				Replicas: pointer.Int32(3),
				Autoscaler: &ManagedSeedAPIServerAutoscaler{
					MinReplicas: pointer.Int32(3),
					MaxReplicas: 6,
				},
			}))
		})
	})

	DescribeTable("#SystemComponentsAllowed",
		func(worker *gardencorev1beta1.Worker, allowsSystemComponents bool) {
			Expect(SystemComponentsAllowed(worker)).To(Equal(allowsSystemComponents))
		},
		Entry("no systemComponents section", &gardencorev1beta1.Worker{}, true),
		Entry("systemComponents.allowed = false", &gardencorev1beta1.Worker{SystemComponents: &gardencorev1beta1.WorkerSystemComponents{Allow: false}}, false),
		Entry("systemComponents.allowed = true", &gardencorev1beta1.Worker{SystemComponents: &gardencorev1beta1.WorkerSystemComponents{Allow: true}}, true),
	)

	DescribeTable("#HibernationIsEnabled",
		func(shoot *gardencorev1beta1.Shoot, hibernated bool) {
			Expect(HibernationIsEnabled(shoot)).To(Equal(hibernated))
		},
		Entry("no hibernation section", &gardencorev1beta1.Shoot{}, false),
		Entry("hibernation.enabled = false", &gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Hibernation: &gardencorev1beta1.Hibernation{Enabled: &falseVar},
			},
		}, false),
		Entry("hibernation.enabled = true", &gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Hibernation: &gardencorev1beta1.Hibernation{Enabled: &trueVar},
			},
		}, true),
	)

	DescribeTable("#ShootWantsClusterAutoscaler",
		func(shoot *gardencorev1beta1.Shoot, wantsAutoscaler bool) {
			actualWantsAutoscaler, err := ShootWantsClusterAutoscaler(shoot)

			Expect(err).NotTo(HaveOccurred())
			Expect(actualWantsAutoscaler).To(Equal(wantsAutoscaler))
		},

		Entry("no workers",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{},
			},
			false),

		Entry("one worker no difference in auto scaler max and min",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{{Name: "foo"}},
					},
				},
			},
			false),

		Entry("one worker with difference in auto scaler max and min",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{{Name: "foo", Minimum: 1, Maximum: 2}},
					},
				},
			},
			true),
	)

	Describe("#ShootWantsVerticalPodAutoscaler", func() {
		var shoot *gardencorev1beta1.Shoot

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{}
		})

		It("should return false", func() {
			shoot.Spec.Kubernetes.VerticalPodAutoscaler = nil
			Expect(ShootWantsVerticalPodAutoscaler(shoot)).To(BeFalse())
		})

		It("should return false", func() {
			shoot.Spec.Kubernetes.VerticalPodAutoscaler = &gardencorev1beta1.VerticalPodAutoscaler{Enabled: false}
			Expect(ShootWantsVerticalPodAutoscaler(shoot)).To(BeFalse())
		})

		It("should return true", func() {
			shoot.Spec.Kubernetes.VerticalPodAutoscaler = &gardencorev1beta1.VerticalPodAutoscaler{Enabled: true}
			Expect(ShootWantsVerticalPodAutoscaler(shoot)).To(BeTrue())
		})
	})

	var (
		unmanagedType = "unmanaged"
		differentType = "foo"
	)

	DescribeTable("#ShootUsesUnmanagedDNS",
		func(dns *gardencorev1beta1.DNS, expectation bool) {
			shoot := &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					DNS: dns,
				},
			}
			Expect(ShootUsesUnmanagedDNS(shoot)).To(Equal(expectation))
		},

		Entry("no dns", nil, false),
		Entry("no dns providers", &gardencorev1beta1.DNS{}, false),
		Entry("dns providers but no type", &gardencorev1beta1.DNS{Providers: []gardencorev1beta1.DNSProvider{{}}}, false),
		Entry("dns providers but different type", &gardencorev1beta1.DNS{Providers: []gardencorev1beta1.DNSProvider{{Type: &differentType}}}, false),
		Entry("dns providers and unmanaged type", &gardencorev1beta1.DNS{Providers: []gardencorev1beta1.DNSProvider{{Type: &unmanagedType}}}, true),
	)

	var profile = gardencorev1beta1.SchedulingProfileBinPacking

	DescribeTable("#ShootSchedulingProfile",
		func(shoot *gardencorev1beta1.Shoot, expected *gardencorev1beta1.SchedulingProfile) {
			Expect(ShootSchedulingProfile(shoot)).To(Equal(expected))
		},
		Entry("no kube-scheduler config",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.24.0",
					},
				},
			},
			nil,
		),
		Entry("kube-scheduler profile is set",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.24.0",
						KubeScheduler: &gardencorev1beta1.KubeSchedulerConfig{
							Profile: &profile,
						},
					},
				},
			},
			&profile,
		),
	)

	DescribeTable("#SeedSettingOwnerChecksEnabled",
		func(settings *gardencorev1beta1.SeedSettings, expected bool) {
			Expect(SeedSettingOwnerChecksEnabled(settings)).To(Equal(expected))
		},

		Entry("no settings", nil, true),
		Entry("no owner checks setting", &gardencorev1beta1.SeedSettings{}, true),
		Entry("owner checks enabled", &gardencorev1beta1.SeedSettings{OwnerChecks: &gardencorev1beta1.SeedSettingOwnerChecks{Enabled: true}}, true),
		Entry("owner checks disabled", &gardencorev1beta1.SeedSettings{OwnerChecks: &gardencorev1beta1.SeedSettingOwnerChecks{Enabled: false}}, false),
	)

	DescribeTable("#SeedSettingDependencyWatchdogWeederEnabled",
		func(settings *gardencorev1beta1.SeedSettings, expected bool) {
			Expect(SeedSettingDependencyWatchdogWeederEnabled(settings)).To(Equal(expected))
		},

		Entry("only dwd weeder set and its enabled", &gardencorev1beta1.SeedSettings{DependencyWatchdog: &gardencorev1beta1.SeedSettingDependencyWatchdog{Weeder: &gardencorev1beta1.SeedSettingDependencyWatchdogWeeder{Enabled: true}}}, true),
		Entry("both dwd weeder and endpoint set and disabled", &gardencorev1beta1.SeedSettings{DependencyWatchdog: &gardencorev1beta1.SeedSettingDependencyWatchdog{Endpoint: &gardencorev1beta1.SeedSettingDependencyWatchdogEndpoint{Enabled: false}, Weeder: &gardencorev1beta1.SeedSettingDependencyWatchdogWeeder{Enabled: false}}}, false),
	)

	DescribeTable("#SeedSettingDependencyWatchdogProberEnabled",
		func(settings *gardencorev1beta1.SeedSettings, expected bool) {
			Expect(SeedSettingDependencyWatchdogProberEnabled(settings)).To(Equal(expected))
		},

		Entry("only dwd prober set and its enabled", &gardencorev1beta1.SeedSettings{DependencyWatchdog: &gardencorev1beta1.SeedSettingDependencyWatchdog{Prober: &gardencorev1beta1.SeedSettingDependencyWatchdogProber{Enabled: true}}}, true),
		Entry("both dwd prober and probe set and disabled", &gardencorev1beta1.SeedSettings{DependencyWatchdog: &gardencorev1beta1.SeedSettingDependencyWatchdog{Prober: &gardencorev1beta1.SeedSettingDependencyWatchdogProber{Enabled: false}, Probe: &gardencorev1beta1.SeedSettingDependencyWatchdogProbe{Enabled: false}}}, false),
	)

	DescribeTable("#SeedSettingTopologyAwareRoutingEnabled",
		func(settings *gardencorev1beta1.SeedSettings, expected bool) {
			Expect(SeedSettingTopologyAwareRoutingEnabled(settings)).To(Equal(expected))
		},

		Entry("no settings", nil, false),
		Entry("no topology-aware routing setting", &gardencorev1beta1.SeedSettings{}, false),
		Entry("topology-aware routing enabled", &gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: true}}, true),
		Entry("topology-aware routing disabled", &gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: false}}, false),
	)

	DescribeTable("#SeedUsesNginxIngressController",
		func(dns gardencorev1beta1.SeedDNS, ingress *gardencorev1beta1.Ingress, expected bool) {
			seed := &gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					DNS:     dns,
					Ingress: ingress,
				},
			}
			Expect(SeedUsesNginxIngressController(seed)).To(Equal(expected))
		},
		Entry("no dns provider", gardencorev1beta1.SeedDNS{}, nil, false),
		Entry("no ingress", gardencorev1beta1.SeedDNS{Provider: &gardencorev1beta1.SeedDNSProvider{}}, nil, false),
		Entry("ingress controller kind is not nginx", gardencorev1beta1.SeedDNS{Provider: &gardencorev1beta1.SeedDNSProvider{}}, &gardencorev1beta1.Ingress{Controller: gardencorev1beta1.IngressController{Kind: "foo"}}, false),
		Entry("ingress controller kind is nginx", gardencorev1beta1.SeedDNS{Provider: &gardencorev1beta1.SeedDNSProvider{}}, &gardencorev1beta1.Ingress{Controller: gardencorev1beta1.IngressController{Kind: "nginx"}}, true),
	)

	Describe("#FindMachineImageVersion", func() {
		var machineImages []gardencorev1beta1.MachineImage

		BeforeEach(func() {
			machineImages = []gardencorev1beta1.MachineImage{
				{
					Name: "coreos",
					Versions: []gardencorev1beta1.MachineImageVersion{
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version: "0.0.2",
							},
						},
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version: "0.0.3",
							},
						},
					},
				},
			}
		})

		It("should find the machine image version when it exists", func() {
			expected := gardencorev1beta1.MachineImageVersion{
				ExpirableVersion: gardencorev1beta1.ExpirableVersion{
					Version: "0.0.3",
				},
			}

			actual, ok := FindMachineImageVersion(machineImages, "coreos", "0.0.3")
			Expect(ok).To(BeTrue())
			Expect(actual).To(Equal(expected))
		})

		It("should return false when machine image with the given name does not exist", func() {
			actual, ok := FindMachineImageVersion(machineImages, "foo", "0.0.3")
			Expect(ok).To(BeFalse())
			Expect(actual).To(Equal(gardencorev1beta1.MachineImageVersion{}))
		})

		It("should return false when machine image version with the given version does not exist", func() {
			actual, ok := FindMachineImageVersion(machineImages, "coreos", "0.0.4")
			Expect(ok).To(BeFalse())
			Expect(actual).To(Equal(gardencorev1beta1.MachineImageVersion{}))
		})
	})

	DescribeTable("#IsAPIServerExposureManaged",
		func(obj metav1.Object, expected bool) {
			Expect(IsAPIServerExposureManaged(obj)).To(Equal(expected))
		},
		Entry("object is nil",
			nil,
			false,
		),
		Entry("label is not present",
			&metav1.ObjectMeta{Labels: map[string]string{
				"foo": "bar",
			}},
			false,
		),
		Entry("label's value is not the same",
			&metav1.ObjectMeta{Labels: map[string]string{
				"core.gardener.cloud/apiserver-exposure": "some-dummy-value",
			}},
			false,
		),
		Entry("label's value is gardener-managed",
			&metav1.ObjectMeta{Labels: map[string]string{
				"core.gardener.cloud/apiserver-exposure": "gardener-managed",
			}},
			true,
		),
	)

	DescribeTable("#FindPrimaryDNSProvider",
		func(providers []gardencorev1beta1.DNSProvider, matcher gomegatypes.GomegaMatcher) {
			Expect(FindPrimaryDNSProvider(providers)).To(matcher)
		},

		Entry("no providers", nil, BeNil()),
		Entry("one non primary provider", []gardencorev1beta1.DNSProvider{
			{Type: pointer.String("provider")},
		}, BeNil()),
		Entry("one primary provider", []gardencorev1beta1.DNSProvider{{Type: pointer.String("provider"),
			Primary: pointer.Bool(true)}}, Equal(&gardencorev1beta1.DNSProvider{Type: pointer.String("provider"), Primary: pointer.Bool(true)})),
		Entry("multiple w/ one primary provider", []gardencorev1beta1.DNSProvider{
			{
				Type: pointer.String("provider2"),
			},
			{
				Type:    pointer.String("provider1"),
				Primary: pointer.Bool(true),
			},
			{
				Type: pointer.String("provider3"),
			},
		}, Equal(&gardencorev1beta1.DNSProvider{Type: pointer.String("provider1"), Primary: pointer.Bool(true)})),
		Entry("multiple w/ multiple primary providers", []gardencorev1beta1.DNSProvider{
			{
				Type:    pointer.String("provider1"),
				Primary: pointer.Bool(true),
			},
			{
				Type:    pointer.String("provider2"),
				Primary: pointer.Bool(true),
			},
			{
				Type: pointer.String("provider3"),
			},
		}, Equal(&gardencorev1beta1.DNSProvider{Type: pointer.String("provider1"), Primary: pointer.Bool(true)})),
	)

	Describe("#ShootMachineImageVersionExists", func() {
		var (
			constraint        gardencorev1beta1.MachineImage
			shootMachineImage gardencorev1beta1.ShootMachineImage
		)

		BeforeEach(func() {
			constraint = gardencorev1beta1.MachineImage{
				Name: "coreos",
				Versions: []gardencorev1beta1.MachineImageVersion{
					{
						ExpirableVersion: gardencorev1beta1.ExpirableVersion{
							Version: "0.0.2",
						},
					},
					{
						ExpirableVersion: gardencorev1beta1.ExpirableVersion{
							Version: "0.0.3",
						},
					},
				},
			}

			shootMachineImage = gardencorev1beta1.ShootMachineImage{
				Name:    "coreos",
				Version: pointer.String("0.0.2"),
			}
		})

		It("should determine that the version exists", func() {
			exists, index := ShootMachineImageVersionExists(constraint, shootMachineImage)
			Expect(exists).To(Equal(trueVar))
			Expect(index).To(Equal(0))
		})

		It("should determine that the version does not exist", func() {
			shootMachineImage.Name = "xy"
			exists, _ := ShootMachineImageVersionExists(constraint, shootMachineImage)
			Expect(exists).To(Equal(false))
		})

		It("should determine that the version does not exist", func() {
			shootMachineImage.Version = pointer.String("0.0.4")
			exists, _ := ShootMachineImageVersionExists(constraint, shootMachineImage)
			Expect(exists).To(Equal(false))
		})
	})

	Describe("Version helper", func() {
		var previewClassification = gardencorev1beta1.ClassificationPreview
		var deprecatedClassification = gardencorev1beta1.ClassificationDeprecated
		var supportedClassification = gardencorev1beta1.ClassificationSupported

		DescribeTable("#GetLatestQualifyingShootMachineImage",
			func(original gardencorev1beta1.MachineImage, expectVersionToBeFound bool, expected *gardencorev1beta1.ShootMachineImage, expectError bool) {
				qualifyingVersionFound, latestVersion, err := GetLatestQualifyingShootMachineImage(original)
				if expectError {
					Expect(err).To(HaveOccurred())
					return
				}
				Expect(err).ToNot(HaveOccurred())
				Expect(qualifyingVersionFound).To(Equal(expectVersionToBeFound))
				Expect(latestVersion).To(Equal(expected))
			},
			Entry("Get latest version",
				gardencorev1beta1.MachineImage{
					Name: "gardenlinux",
					Versions: []gardencorev1beta1.MachineImageVersion{
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version: "1.17.1",
							},
						},
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version: "1.15.0",
							},
						},
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version: "1.14.3",
							},
						},
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version: "1.13.1",
							},
						},
					},
				},
				true,
				&gardencorev1beta1.ShootMachineImage{
					Name:    "gardenlinux",
					Version: pointer.String("1.17.1"),
				},
				false,
			),
			Entry("Expect no qualifying version to be found - machine image has only versions in preview and expired versions",
				gardencorev1beta1.MachineImage{
					Name: "gardenlinux",
					Versions: []gardencorev1beta1.MachineImageVersion{
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version:        "1.17.1",
								Classification: &previewClassification,
							},
						},
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version:        "1.15.0",
								Classification: &previewClassification,
							},
						},
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version:        "1.14.3",
								ExpirationDate: &expirationDateInThePast,
							},
						},
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version:        "1.13.1",
								ExpirationDate: &expirationDateInThePast,
							},
						},
					},
				},
				false,
				nil,
				false,
			),
			Entry("Expect older but supported version to be preferred over newer but deprecated one",
				gardencorev1beta1.MachineImage{
					Name: "gardenlinux",
					Versions: []gardencorev1beta1.MachineImageVersion{
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version:        "1.17.1",
								Classification: &deprecatedClassification,
							},
						},
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version:        "1.16.1",
								Classification: &supportedClassification,
							},
						},
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version:        "1.15.0",
								Classification: &previewClassification,
							},
						},
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version:        "1.14.3",
								ExpirationDate: &expirationDateInThePast,
							},
						},
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version:        "1.13.1",
								ExpirationDate: &expirationDateInThePast,
							},
						},
					},
				},
				true,
				&gardencorev1beta1.ShootMachineImage{
					Name:    "gardenlinux",
					Version: pointer.String("1.16.1"),
				},
				false,
			),
			Entry("Expect latest deprecated version to be selected when there is no supported version",
				gardencorev1beta1.MachineImage{
					Name: "gardenlinux",
					Versions: []gardencorev1beta1.MachineImageVersion{
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version:        "1.17.3",
								Classification: &previewClassification,
							},
						},
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version:        "1.17.2",
								ExpirationDate: &expirationDateInThePast,
							},
						},
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version:        "1.17.1",
								Classification: &deprecatedClassification,
							},
						},
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version:        "1.16.1",
								Classification: &deprecatedClassification,
							},
						},
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version:        "1.15.0",
								Classification: &previewClassification,
							},
						},
						{
							ExpirableVersion: gardencorev1beta1.ExpirableVersion{
								Version:        "1.14.3",
								ExpirationDate: &expirationDateInThePast,
							},
						},
					},
				},
				true,
				&gardencorev1beta1.ShootMachineImage{
					Name:    "gardenlinux",
					Version: pointer.String("1.17.1"),
				},
				false,
			),
		)

		DescribeTable("#GetLatestQualifyingVersion",
			func(original []gardencorev1beta1.ExpirableVersion, expectVersionToBeFound bool, expected *gardencorev1beta1.ExpirableVersion, expectError bool) {
				qualifyingVersionFound, latestVersion, err := GetLatestQualifyingVersion(original, nil)
				if expectError {
					Expect(err).To(HaveOccurred())
					return
				}
				Expect(err).ToNot(HaveOccurred())
				Expect(qualifyingVersionFound).To(Equal(expectVersionToBeFound))
				Expect(latestVersion).To(Equal(expected))
			},
			Entry("Get latest non-preview version",
				[]gardencorev1beta1.ExpirableVersion{
					{
						Version:        "1.17.2",
						Classification: &previewClassification,
					},
					{
						Version: "1.17.1",
					},
					{
						Version: "1.15.0",
					},
					{
						Version: "1.14.3",
					},
					{
						Version: "1.13.1",
					},
				},
				true,
				&gardencorev1beta1.ExpirableVersion{
					Version: "1.17.1",
				},
				false,
			),
			Entry("Expect no qualifying version to be found - no latest version could be found",
				[]gardencorev1beta1.ExpirableVersion{},
				false,
				nil,
				false,
			),
			Entry("Expect error, because contains invalid semVer",
				[]gardencorev1beta1.ExpirableVersion{
					{
						Version: "1.213123xx",
					},
				},
				false,
				nil,
				true,
			),
		)

		Describe("#Kubernetes Version Helper", func() {
			DescribeTable("#GetKubernetesVersionForPatchUpdate",
				func(currentVersion string, cloudProfileVersions []gardencorev1beta1.ExpirableVersion, expectedVersion string, qualifyingVersionFound bool) {
					cloudProfile := gardencorev1beta1.CloudProfile{
						Spec: gardencorev1beta1.CloudProfileSpec{
							Kubernetes: gardencorev1beta1.KubernetesSettings{
								Versions: cloudProfileVersions,
							},
						},
					}
					ok, newVersion, err := GetKubernetesVersionForPatchUpdate(&cloudProfile, currentVersion)
					Expect(err).ToNot(HaveOccurred())
					Expect(ok).To(Equal(qualifyingVersionFound))
					Expect(newVersion).To(Equal(expectedVersion))
				},
				Entry("Do not consider preview versions for patch update.",
					"1.12.2",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{Version: "1.14.4"},
						{
							Version:        "1.12.9",
							Classification: &previewClassification,
						},
						{
							Version:        "1.12.4",
							Classification: &previewClassification,
						},
						// latest qualifying version for updating version 1.12.2
						{Version: "1.12.3"},
						{Version: "1.12.2"},
					},
					"1.12.3",
					true,
				),
				Entry("Do not consider expired versions for patch update.",
					"1.12.2",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{Version: "1.14.4"},
						{
							Version:        "1.12.9",
							ExpirationDate: &expirationDateInThePast,
						},
						{
							Version:        "1.12.4",
							ExpirationDate: &expirationDateInThePast,
						},
						// latest qualifying version for updating version 1.12.2
						{Version: "1.12.3"},
						{Version: "1.12.2"},
					},
					"1.12.3",
					true,
				),
				Entry("Should not find qualifying version - no higher version available that is not expired or in preview.",
					"1.12.2",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{Version: "1.14.4"},
						{
							Version:        "1.12.9",
							ExpirationDate: &expirationDateInThePast,
						},
						{
							Version:        "1.12.4",
							Classification: &previewClassification,
						},
						{Version: "1.12.2"},
					},
					"",
					false,
				),
				Entry("Should not find qualifying version - is already highest version of minor.",
					"1.12.2",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{Version: "1.14.4"},
						{Version: "1.12.2"},
					},
					"",
					false,
				),
				Entry("Should not find qualifying version - is already on latest version of latest minor.",
					"1.15.1",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{Version: "1.14.4"},
						{Version: "1.12.2"},
					},
					"",
					false,
				),
			)

			DescribeTable("#GetKubernetesVersionForMinorUpdate",
				func(currentVersion string, cloudProfileVersions []gardencorev1beta1.ExpirableVersion, expectedVersion string, qualifyingVersionFound bool) {
					cloudProfile := gardencorev1beta1.CloudProfile{
						Spec: gardencorev1beta1.CloudProfileSpec{
							Kubernetes: gardencorev1beta1.KubernetesSettings{
								Versions: cloudProfileVersions,
							},
						},
					}
					ok, newVersion, err := GetKubernetesVersionForMinorUpdate(&cloudProfile, currentVersion)
					Expect(err).ToNot(HaveOccurred())
					Expect(ok).To(Equal(qualifyingVersionFound))
					Expect(newVersion).To(Equal(expectedVersion))
				},
				Entry("Do not consider preview versions of the consecutive minor version.",
					"1.11.3",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{
							Version:        "1.12.9",
							Classification: &previewClassification,
						},
						{
							Version:        "1.12.4",
							Classification: &previewClassification,
						},
						// latest qualifying version for minor version update for version 1.11.3
						{Version: "1.12.3"},
						{Version: "1.12.2"},
						{Version: "1.11.3"},
					},
					"1.12.3",
					true,
				),
				Entry("Should find qualifying version - latest non-expired version of the consecutive minor version.",
					"1.11.3",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{
							Version:        "1.12.9",
							ExpirationDate: &expirationDateInThePast,
						},
						{
							Version:        "1.12.4",
							ExpirationDate: &expirationDateInThePast,
						},
						// latest qualifying version for updating version 1.11.3
						{Version: "1.12.3"},
						{Version: "1.12.2"},
						{Version: "1.11.3"},
						{Version: "1.10.1"},
						{Version: "1.09.0"},
					},
					"1.12.3",
					true,
				),
				// check that multiple consecutive minor versions are possible
				Entry("Should find qualifying version if there are only expired versions available in the consecutive minor version - pick latest expired version of that minor.",
					"1.11.3",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						// latest qualifying version for updating version 1.11.3
						{
							Version:        "1.12.9",
							ExpirationDate: &expirationDateInThePast,
						},
						{
							Version:        "1.12.4",
							ExpirationDate: &expirationDateInThePast,
						},
						{Version: "1.11.3"},
					},
					"1.12.9",
					true,
				),
				Entry("Should not find qualifying version - there is no consecutive minor version available.",
					"1.10.3",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{
							Version:        "1.12.9",
							ExpirationDate: &expirationDateInThePast,
						},
						{
							Version:        "1.12.4",
							ExpirationDate: &expirationDateInThePast,
						},
						{Version: "1.12.3"},
						{Version: "1.12.2"},
						{Version: "1.10.3"},
					},
					"",
					false,
				),
				Entry("Should not find qualifying version - already on latest minor version.",
					"1.15.1",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{Version: "1.14.4"},
						{Version: "1.12.2"},
					},
					"",
					false,
				),
				Entry("Should not find qualifying version - is already on latest version of latest minor version.",
					"1.15.1",
					[]gardencorev1beta1.ExpirableVersion{
						{Version: "1.15.1"},
						{Version: "1.15.0"},
						{Version: "1.14.4"},
						{Version: "1.12.2"},
					},
					"",
					false,
				),
			)
			DescribeTable("Test version filter predicates",
				func(predicate VersionPredicate, version *semver.Version, expirableVersion gardencorev1beta1.ExpirableVersion, expectFilterVersion, expectError bool) {
					shouldFilter, err := predicate(expirableVersion, version)
					if expectError {
						Expect(err).To(HaveOccurred())
						return
					}
					Expect(err).ToNot(HaveOccurred())
					Expect(shouldFilter).To(Equal(expectFilterVersion))
				},

				// #FilterDifferentMajorMinorVersion
				Entry("Should filter version - has not the same major.minor.",
					FilterDifferentMajorMinorVersion(*semver.MustParse("1.2.0")),
					semver.MustParse("1.1.1"),
					gardencorev1beta1.ExpirableVersion{},
					true,
					false,
				),
				Entry("Should filter version - version has same major.minor but is lower",
					FilterDifferentMajorMinorVersion(*semver.MustParse("1.1.2")),
					semver.MustParse("1.1.1"),
					gardencorev1beta1.ExpirableVersion{},
					true,
					false,
				),
				Entry("Should not filter version - has the same major.minor.",
					FilterDifferentMajorMinorVersion(*semver.MustParse("1.1.0")),
					semver.MustParse("1.1.1"),
					gardencorev1beta1.ExpirableVersion{},
					false,
					false,
				),

				// #FilterNonConsecutiveMinorVersion
				Entry("Should filter version - has not the consecutive minor version.",
					FilterNonConsecutiveMinorVersion(*semver.MustParse("1.3.0")),
					semver.MustParse("1.1.1"),
					gardencorev1beta1.ExpirableVersion{},
					true,
					false,
				),
				Entry("Should filter version - has the same minor version.",
					FilterNonConsecutiveMinorVersion(*semver.MustParse("1.1.0")),
					semver.MustParse("1.1.1"),
					gardencorev1beta1.ExpirableVersion{},
					true,
					false,
				),
				Entry("Should not filter version - has consecutive minor.",
					FilterNonConsecutiveMinorVersion(*semver.MustParse("1.1.0")),
					semver.MustParse("1.2.0"),
					gardencorev1beta1.ExpirableVersion{},
					false,
					false,
				),

				// #FilterSameVersion
				Entry("Should filter version.",
					FilterSameVersion(*semver.MustParse("1.1.1")),
					semver.MustParse("1.1.1"),
					gardencorev1beta1.ExpirableVersion{},
					true,
					false,
				),
				Entry("Should not filter version.",
					FilterSameVersion(*semver.MustParse("1.1.1")),
					semver.MustParse("1.1.2"),
					gardencorev1beta1.ExpirableVersion{},
					false,
					false,
				),

				// #FilterExpiredVersion
				Entry("Should filter expired version.",
					FilterExpiredVersion(),
					nil,
					gardencorev1beta1.ExpirableVersion{
						ExpirationDate: &expirationDateInThePast,
					},
					true,
					false,
				),
				Entry("Should not filter version - expiration date is not expired",
					FilterExpiredVersion(),
					nil,
					gardencorev1beta1.ExpirableVersion{
						ExpirationDate: &metav1.Time{Time: time.Now().Add(time.Hour)},
					},
					false,
					false,
				),
				Entry("Should not filter version.",
					FilterExpiredVersion(),
					nil,
					gardencorev1beta1.ExpirableVersion{},
					false,
					false,
				),
				// #FilterDeprecatedVersion
				Entry("Should filter version - version is deprecated",
					FilterDeprecatedVersion(),
					nil,
					gardencorev1beta1.ExpirableVersion{Classification: &deprecatedClassification},
					true,
					false,
				),
				Entry("Should not filter version - version has preview classification",
					FilterDeprecatedVersion(),
					nil,
					gardencorev1beta1.ExpirableVersion{Classification: &previewClassification},
					false,
					false,
				),
				Entry("Should not filter version - version has supported classification",
					FilterDeprecatedVersion(),
					nil,
					gardencorev1beta1.ExpirableVersion{Classification: &supportedClassification},
					false,
					false,
				),
				Entry("Should not filter version - version has no classification",
					FilterDeprecatedVersion(),
					nil,
					gardencorev1beta1.ExpirableVersion{},
					false,
					false,
				),
				// #FilterLowerVersion
				Entry("Should filter version - version is lower",
					FilterLowerVersion(*semver.MustParse("1.1.1")),
					semver.MustParse("1.1.0"),
					gardencorev1beta1.ExpirableVersion{},
					true,
					false,
				),
				Entry("Should not filter version - version is higher / equal",
					FilterLowerVersion(*semver.MustParse("1.1.1")),
					semver.MustParse("1.1.2"),
					gardencorev1beta1.ExpirableVersion{},
					false,
					false,
				),
			)
		})

		DescribeTable("#UpsertLastError",
			func(lastErrors []gardencorev1beta1.LastError, lastError gardencorev1beta1.LastError, expected []gardencorev1beta1.LastError) {
				Expect(UpsertLastError(lastErrors, lastError)).To(Equal(expected))
			},

			Entry(
				"insert",
				[]gardencorev1beta1.LastError{
					{},
					{TaskID: pointer.String("bar")},
				},
				gardencorev1beta1.LastError{TaskID: pointer.String("foo"), Description: "error"},
				[]gardencorev1beta1.LastError{
					{},
					{TaskID: pointer.String("bar")},
					{TaskID: pointer.String("foo"), Description: "error"},
				},
			),
			Entry(
				"update",
				[]gardencorev1beta1.LastError{
					{},
					{TaskID: pointer.String("foo"), Description: "error"},
					{TaskID: pointer.String("bar")},
				},
				gardencorev1beta1.LastError{TaskID: pointer.String("foo"), Description: "new-error"},
				[]gardencorev1beta1.LastError{
					{},
					{TaskID: pointer.String("foo"), Description: "new-error"},
					{TaskID: pointer.String("bar")},
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
					{TaskID: pointer.String("bar")},
				},
				"foo",
				[]gardencorev1beta1.LastError{
					{},
					{TaskID: pointer.String("bar")},
				},
			),
			Entry(
				"task id found",
				[]gardencorev1beta1.LastError{
					{},
					{TaskID: pointer.String("foo")},
					{TaskID: pointer.String("bar")},
				},
				"foo",
				[]gardencorev1beta1.LastError{
					{},
					{TaskID: pointer.String("bar")},
				},
			),
		)
	})

	Describe("ShootItems", func() {
		Describe("#Union", func() {
			It("tests if provided two sets of shoot slices will return ", func() {
				shootList1 := gardencorev1beta1.ShootList{
					Items: []gardencorev1beta1.Shoot{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "shoot1",
								Namespace: "namespace1",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "shoot2",
								Namespace: "namespace1",
							},
						}, {
							ObjectMeta: metav1.ObjectMeta{
								Name:      "shoot3",
								Namespace: "namespace2",
							},
						},
					},
				}

				shootList2 := gardencorev1beta1.ShootList{
					Items: []gardencorev1beta1.Shoot{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "shoot2",
								Namespace: "namespace2",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "shoot1",
								Namespace: "namespace1",
							},
						}, {
							ObjectMeta: metav1.ObjectMeta{
								Name:      "shoot3",
								Namespace: "namespace3",
							},
						},
					},
				}

				s := ShootItems(shootList1)
				s2 := ShootItems(shootList2)
				shootSet := s.Union(&s2)

				Expect(len(shootSet)).To(Equal(5))
			})

			It("should not fail if one of the lists is empty", func() {
				shootList1 := gardencorev1beta1.ShootList{
					Items: []gardencorev1beta1.Shoot{},
				}

				shootList2 := gardencorev1beta1.ShootList{
					Items: []gardencorev1beta1.Shoot{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "shoot2",
								Namespace: "namespace2",
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "shoot1",
								Namespace: "namespace1",
							},
						}, {
							ObjectMeta: metav1.ObjectMeta{
								Name:      "shoot3",
								Namespace: "namespace3",
							},
						},
					},
				}

				s := ShootItems(shootList1)
				s2 := ShootItems(shootList2)
				shootSet := s.Union(&s2)
				Expect(len(shootSet)).To(Equal(3))

				shootSet2 := s2.Union(&s)
				Expect(len(shootSet)).To(Equal(3))
				Expect(shootSet).To(ConsistOf(shootSet2))

			})
		})

		It("should not fail if no items", func() {
			shootList1 := gardencorev1beta1.ShootList{}

			shootList2 := gardencorev1beta1.ShootList{}

			s := ShootItems(shootList1)
			s2 := ShootItems(shootList2)
			shootSet := s.Union(&s2)
			Expect(len(shootSet)).To(Equal(0))
		})
	})

	Describe("#GetPurpose", func() {
		var shoot *gardencorev1beta1.Shoot

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{},
			}
		})

		It("should get default purpose if not defined", func() {
			purpose := GetPurpose(shoot)
			Expect(purpose).To(Equal(gardencorev1beta1.ShootPurposeEvaluation))
		})

		It("should get purpose", func() {
			shootPurpose := gardencorev1beta1.ShootPurposeProduction
			shoot.Spec.Purpose = &shootPurpose
			purpose := GetPurpose(shoot)
			Expect(purpose).To(Equal(shootPurpose))
		})
	})

	Context("Shoot Alerts", func() {
		var shoot *gardencorev1beta1.Shoot

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{}
		})

		Describe("#ShootIgnoresAlerts", func() {
			It("should not ignore alerts because no annotations given", func() {
				Expect(ShootIgnoresAlerts(shoot)).To(BeFalse())
			})
			It("should not ignore alerts because annotation is not given", func() {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "foo", "bar")
				Expect(ShootIgnoresAlerts(shoot)).To(BeFalse())
			})
			It("should not ignore alerts because annotation value is false", func() {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.AnnotationShootIgnoreAlerts, "false")
				Expect(ShootIgnoresAlerts(shoot)).To(BeFalse())
			})
			It("should ignore alerts because annotation value is true", func() {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.AnnotationShootIgnoreAlerts, "true")
				Expect(ShootIgnoresAlerts(shoot)).To(BeTrue())
			})
		})

		Describe("#ShootWantsAlertManager", func() {
			It("should not want alert manager because alerts are ignored", func() {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.AnnotationShootIgnoreAlerts, "true")
				Expect(ShootWantsAlertManager(shoot)).To(BeFalse())
			})
			It("should not want alert manager because of missing monitoring configuration", func() {
				Expect(ShootWantsAlertManager(shoot)).To(BeFalse())
			})
			It("should not want alert manager because of missing alerting configuration", func() {
				shoot.Spec = gardencorev1beta1.ShootSpec{
					Monitoring: &gardencorev1beta1.Monitoring{},
				}
				Expect(ShootWantsAlertManager(shoot)).To(BeFalse())
			})
			It("should not want alert manager because of missing email configuration", func() {
				shoot.Spec = gardencorev1beta1.ShootSpec{
					Monitoring: &gardencorev1beta1.Monitoring{
						Alerting: &gardencorev1beta1.Alerting{},
					},
				}
				Expect(ShootWantsAlertManager(shoot)).To(BeFalse())
			})
			It("should want alert manager", func() {
				shoot.Spec = gardencorev1beta1.ShootSpec{
					Monitoring: &gardencorev1beta1.Monitoring{
						Alerting: &gardencorev1beta1.Alerting{
							EmailReceivers: []string{"operators@gardener.clou"},
						},
					},
				}
				Expect(ShootWantsAlertManager(shoot)).To(BeTrue())
			})
		})
	})

	DescribeTable("#KubernetesDashboardEnabled",
		func(addons *gardencorev1beta1.Addons, matcher gomegatypes.GomegaMatcher) {
			Expect(KubernetesDashboardEnabled(addons)).To(matcher)
		},

		Entry("addons nil", nil, BeFalse()),
		Entry("kubernetesDashboard nil", &gardencorev1beta1.Addons{}, BeFalse()),
		Entry("kubernetesDashboard disabled", &gardencorev1beta1.Addons{KubernetesDashboard: &gardencorev1beta1.KubernetesDashboard{Addon: gardencorev1beta1.Addon{Enabled: false}}}, BeFalse()),
		Entry("kubernetesDashboard enabled", &gardencorev1beta1.Addons{KubernetesDashboard: &gardencorev1beta1.KubernetesDashboard{Addon: gardencorev1beta1.Addon{Enabled: true}}}, BeTrue()),
	)

	DescribeTable("#NginxIngressEnabled",
		func(addons *gardencorev1beta1.Addons, matcher gomegatypes.GomegaMatcher) {
			Expect(NginxIngressEnabled(addons)).To(matcher)
		},

		Entry("addons nil", nil, BeFalse()),
		Entry("nginxIngress nil", &gardencorev1beta1.Addons{}, BeFalse()),
		Entry("nginxIngress disabled", &gardencorev1beta1.Addons{NginxIngress: &gardencorev1beta1.NginxIngress{Addon: gardencorev1beta1.Addon{Enabled: false}}}, BeFalse()),
		Entry("nginxIngress enabled", &gardencorev1beta1.Addons{NginxIngress: &gardencorev1beta1.NginxIngress{Addon: gardencorev1beta1.Addon{Enabled: true}}}, BeTrue()),
	)

	DescribeTable("#KubeProxyEnabled",
		func(kubeProxy *gardencorev1beta1.KubeProxyConfig, matcher gomegatypes.GomegaMatcher) {
			Expect(KubeProxyEnabled(kubeProxy)).To(matcher)
		},

		Entry("kubeProxy nil", nil, BeFalse()),
		Entry("kubeProxy empty", &gardencorev1beta1.KubeProxyConfig{}, BeFalse()),
		Entry("kubeProxy disabled", &gardencorev1beta1.KubeProxyConfig{Enabled: pointer.Bool(false)}, BeFalse()),
		Entry("kubeProxy enabled", &gardencorev1beta1.KubeProxyConfig{Enabled: pointer.Bool(true)}, BeTrue()),
	)

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

	DescribeTable("#SeedBackupSecretRefEqual",
		func(oldBackup, newBackup *gardencorev1beta1.SeedBackup, matcher gomegatypes.GomegaMatcher) {
			Expect(SeedBackupSecretRefEqual(oldBackup, newBackup)).To(matcher)
		},

		Entry("both nil", nil, nil, BeTrue()),
		Entry("old nil, new empty", nil, &gardencorev1beta1.SeedBackup{}, BeTrue()),
		Entry("old empty, new nil", &gardencorev1beta1.SeedBackup{}, nil, BeTrue()),
		Entry("both empty", &gardencorev1beta1.SeedBackup{}, &gardencorev1beta1.SeedBackup{}, BeTrue()),
		Entry("difference", &gardencorev1beta1.SeedBackup{SecretRef: corev1.SecretReference{Name: "foo", Namespace: "bar"}}, &gardencorev1beta1.SeedBackup{SecretRef: corev1.SecretReference{Name: "bar", Namespace: "foo"}}, BeFalse()),
		Entry("equality", &gardencorev1beta1.SeedBackup{SecretRef: corev1.SecretReference{Name: "foo", Namespace: "bar"}}, &gardencorev1beta1.SeedBackup{SecretRef: corev1.SecretReference{Name: "foo", Namespace: "bar"}}, BeTrue()),
	)

	DescribeTable("#ShootDNSProviderSecretNamesEqual",
		func(oldDNS, newDNS *gardencorev1beta1.DNS, matcher gomegatypes.GomegaMatcher) {
			Expect(ShootDNSProviderSecretNamesEqual(oldDNS, newDNS)).To(matcher)
		},

		Entry("both nil", nil, nil, BeTrue()),
		Entry("old nil, new w/o secret names", nil, &gardencorev1beta1.DNS{}, BeTrue()),
		Entry("old w/o secret names, new nil", &gardencorev1beta1.DNS{}, nil, BeTrue()),
		Entry("difference due to old", &gardencorev1beta1.DNS{}, &gardencorev1beta1.DNS{Providers: []gardencorev1beta1.DNSProvider{{SecretName: pointer.String("foo")}}}, BeFalse()),
		Entry("difference due to new", &gardencorev1beta1.DNS{Providers: []gardencorev1beta1.DNSProvider{{SecretName: pointer.String("foo")}}}, &gardencorev1beta1.DNS{}, BeFalse()),
		Entry("equality", &gardencorev1beta1.DNS{Providers: []gardencorev1beta1.DNSProvider{{SecretName: pointer.String("foo")}}}, &gardencorev1beta1.DNS{Providers: []gardencorev1beta1.DNSProvider{{SecretName: pointer.String("foo")}}}, BeTrue()),
	)

	DescribeTable("#ShootSecretResourceReferencesEqual",
		func(oldResources, newResources []gardencorev1beta1.NamedResourceReference, matcher gomegatypes.GomegaMatcher) {
			Expect(ShootSecretResourceReferencesEqual(oldResources, newResources)).To(matcher)
		},

		Entry("both nil", nil, nil, BeTrue()),
		Entry("old empty, new w/o secrets", []gardencorev1beta1.NamedResourceReference{}, []gardencorev1beta1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{Name: "foo"}}}, BeTrue()),
		Entry("old empty, new w/ secrets", []gardencorev1beta1.NamedResourceReference{}, []gardencorev1beta1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo"}}}, BeFalse()),
		Entry("old w/o secrets, new empty", []gardencorev1beta1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{Name: "foo"}}}, []gardencorev1beta1.NamedResourceReference{}, BeTrue()),
		Entry("old w/ secrets, new empty", []gardencorev1beta1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo"}}}, []gardencorev1beta1.NamedResourceReference{}, BeFalse()),
		Entry("difference", []gardencorev1beta1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo"}}}, []gardencorev1beta1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "bar"}}}, BeFalse()),
		Entry("difference because no secret", []gardencorev1beta1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo"}}}, []gardencorev1beta1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "ConfigMap", Name: "foo"}}}, BeFalse()),
		Entry("equality", []gardencorev1beta1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo"}}}, []gardencorev1beta1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo"}}}, BeTrue()),
	)

	DescribeTable("#AnonymousAuthenticationEnabled",
		func(kubeAPIServerConfig *gardencorev1beta1.KubeAPIServerConfig, wantsAnonymousAuth bool) {
			actualWantsAnonymousAuth := AnonymousAuthenticationEnabled(kubeAPIServerConfig)
			Expect(actualWantsAnonymousAuth).To(Equal(wantsAnonymousAuth))
		},

		Entry("no kubeapiserver configuration", nil, false),
		Entry("field not set", &gardencorev1beta1.KubeAPIServerConfig{}, false),
		Entry("explicitly enabled", &gardencorev1beta1.KubeAPIServerConfig{EnableAnonymousAuthentication: &trueVar}, true),
		Entry("explicitly disabled", &gardencorev1beta1.KubeAPIServerConfig{EnableAnonymousAuthentication: &falseVar}, false),
	)

	Describe("GetShootAuditPolicyConfigMapName", func() {
		test := func(description string, config *gardencorev1beta1.KubeAPIServerConfig, expectedName string) {
			It(description, Offset(1), func() {
				Expect(GetShootAuditPolicyConfigMapName(config)).To(Equal(expectedName))
			})
		}

		test("KubeAPIServerConfig = nil", nil, "")
		test("AuditConfig = nil", &gardencorev1beta1.KubeAPIServerConfig{}, "")
		test("AuditPolicy = nil", &gardencorev1beta1.KubeAPIServerConfig{
			AuditConfig: &gardencorev1beta1.AuditConfig{},
		}, "")
		test("ConfigMapRef = nil", &gardencorev1beta1.KubeAPIServerConfig{
			AuditConfig: &gardencorev1beta1.AuditConfig{
				AuditPolicy: &gardencorev1beta1.AuditPolicy{},
			},
		}, "")
		test("ConfigMapRef set", &gardencorev1beta1.KubeAPIServerConfig{
			AuditConfig: &gardencorev1beta1.AuditConfig{
				AuditPolicy: &gardencorev1beta1.AuditPolicy{
					ConfigMapRef: &corev1.ObjectReference{Name: "foo"},
				},
			},
		}, "foo")
	})

	Describe("GetShootAuditPolicyConfigMapRef", func() {
		test := func(description string, config *gardencorev1beta1.KubeAPIServerConfig, expectedRef *corev1.ObjectReference) {
			It(description, Offset(1), func() {
				Expect(GetShootAuditPolicyConfigMapRef(config)).To(Equal(expectedRef))
			})
		}

		test("KubeAPIServerConfig = nil", nil, nil)
		test("AuditConfig = nil", &gardencorev1beta1.KubeAPIServerConfig{}, nil)
		test("AuditPolicy = nil", &gardencorev1beta1.KubeAPIServerConfig{
			AuditConfig: &gardencorev1beta1.AuditConfig{},
		}, nil)
		test("ConfigMapRef = nil", &gardencorev1beta1.KubeAPIServerConfig{
			AuditConfig: &gardencorev1beta1.AuditConfig{
				AuditPolicy: &gardencorev1beta1.AuditPolicy{},
			},
		}, nil)
		test("ConfigMapRef set", &gardencorev1beta1.KubeAPIServerConfig{
			AuditConfig: &gardencorev1beta1.AuditConfig{
				AuditPolicy: &gardencorev1beta1.AuditPolicy{
					ConfigMapRef: &corev1.ObjectReference{Name: "foo"},
				},
			},
		}, &corev1.ObjectReference{Name: "foo"})
	})

	Describe("#CalculateSeedUsage", func() {
		type shootCase struct {
			specSeedName, statusSeedName string
		}

		test := func(shoots []shootCase, expectedUsage map[string]int) {
			var shootList []gardencorev1beta1.Shoot

			for i, shoot := range shoots {
				s := gardencorev1beta1.Shoot{}
				s.Name = fmt.Sprintf("shoot-%d", i)
				if shoot.specSeedName != "" {
					s.Spec.SeedName = pointer.String(shoot.specSeedName)
				}
				if shoot.statusSeedName != "" {
					s.Status.SeedName = pointer.String(shoot.statusSeedName)
				}
				shootList = append(shootList, s)
			}

			ExpectWithOffset(1, CalculateSeedUsage(shootList)).To(Equal(expectedUsage))
		}

		It("no shoots", func() {
			test([]shootCase{}, map[string]int{})
		})
		It("shoot with both fields unset", func() {
			test([]shootCase{{}}, map[string]int{})
		})
		It("shoot with only spec set", func() {
			test([]shootCase{{specSeedName: "seed"}}, map[string]int{"seed": 1})
		})
		It("shoot with only status set", func() {
			test([]shootCase{{statusSeedName: "seed"}}, map[string]int{"seed": 1})
		})
		It("shoot with both fields set to same seed", func() {
			test([]shootCase{{specSeedName: "seed", statusSeedName: "seed"}}, map[string]int{"seed": 1})
		})
		It("shoot with fields set to different seeds", func() {
			test([]shootCase{{specSeedName: "seed", statusSeedName: "seed2"}}, map[string]int{"seed": 1, "seed2": 1})
		})
		It("multiple shoots", func() {
			test([]shootCase{
				{},
				{specSeedName: "seed", statusSeedName: "seed2"},
				{specSeedName: "seed2", statusSeedName: "seed2"},
				{specSeedName: "seed3", statusSeedName: "seed2"},
				{specSeedName: "seed3", statusSeedName: "seed4"},
			}, map[string]int{"seed": 1, "seed2": 3, "seed3": 2, "seed4": 1})
		})
	})

	DescribeTable("#CalculateEffectiveKubernetesVersion",
		func(controlPlaneVersion *semver.Version, workerKubernetes *gardencorev1beta1.WorkerKubernetes, expectedRes *semver.Version) {
			res, err := CalculateEffectiveKubernetesVersion(controlPlaneVersion, workerKubernetes)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(expectedRes))
		},

		Entry("workerKubernetes = nil", semver.MustParse("1.2.3"), nil, semver.MustParse("1.2.3")),
		Entry("workerKubernetes.version = nil", semver.MustParse("1.2.3"), &gardencorev1beta1.WorkerKubernetes{}, semver.MustParse("1.2.3")),
		Entry("workerKubernetes.version != nil", semver.MustParse("1.2.3"), &gardencorev1beta1.WorkerKubernetes{Version: pointer.String("4.5.6")}, semver.MustParse("4.5.6")),
	)

	DescribeTable("#GetSecretBindingTypes",
		func(secretBinding *gardencorev1beta1.SecretBinding, expected []string) {
			actual := GetSecretBindingTypes(secretBinding)
			Expect(actual).To(Equal(expected))
		},

		Entry("with single-value provider type", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo"}}, []string{"foo"}),
		Entry("with multi-value provider type", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo,bar,baz"}}, []string{"foo", "bar", "baz"}),
	)

	DescribeTable("#SecretBindingHasType",
		func(secretBinding *gardencorev1beta1.SecretBinding, toFind string, expected bool) {
			actual := SecretBindingHasType(secretBinding, toFind)
			Expect(actual).To(Equal(expected))
		},

		Entry("with empty provider field", &gardencorev1beta1.SecretBinding{}, "foo", false),
		Entry("when single-value provider type equals to the given type", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo"}}, "foo", true),
		Entry("when single-value provider type does not match the given type", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo"}}, "bar", false),
		Entry("when multi-value provider type contains the given type", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo,bar"}}, "bar", true),
		Entry("when multi-value provider type does not contain the given type", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo,bar"}}, "baz", false),
	)

	DescribeTable("#AddTypeToSecretBinding",
		func(secretBinding *gardencorev1beta1.SecretBinding, toAdd, expected string) {
			AddTypeToSecretBinding(secretBinding, toAdd)
			Expect(secretBinding.Provider.Type).To(Equal(expected))
		},

		Entry("with empty provider field", &gardencorev1beta1.SecretBinding{}, "foo", "foo"),
		Entry("when single-value provider type already exists", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo"}}, "foo", "foo"),
		Entry("when single-value provider type does not exist", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo"}}, "bar", "foo,bar"),
		Entry("when multi-value provider type already exists", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo,bar"}}, "foo", "foo,bar"),
		Entry("when multi-value provider type does not exist", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo,bar"}}, "baz", "foo,bar,baz"),
	)

	DescribeTable("#IsCoreDNSAutoscalingModeUsed",
		func(systemComponents *gardencorev1beta1.SystemComponents, autoscalingMode gardencorev1beta1.CoreDNSAutoscalingMode, expected bool) {
			Expect(IsCoreDNSAutoscalingModeUsed(systemComponents, autoscalingMode)).To(Equal(expected))
		},

		Entry("with nil (cluster-proportional)", nil, gardencorev1beta1.CoreDNSAutoscalingModeClusterProportional, false),
		Entry("with nil (horizontal)", nil, gardencorev1beta1.CoreDNSAutoscalingModeHorizontal, true),
		Entry("with empty system components (cluster-proportional)", &gardencorev1beta1.SystemComponents{}, gardencorev1beta1.CoreDNSAutoscalingModeClusterProportional, false),
		Entry("with empty system components (horizontal)", &gardencorev1beta1.SystemComponents{}, gardencorev1beta1.CoreDNSAutoscalingModeHorizontal, true),
		Entry("with empty core dns (cluster-proportional)", &gardencorev1beta1.SystemComponents{CoreDNS: &gardencorev1beta1.CoreDNS{}}, gardencorev1beta1.CoreDNSAutoscalingModeClusterProportional, false),
		Entry("with empty core dns (horizontal)", &gardencorev1beta1.SystemComponents{CoreDNS: &gardencorev1beta1.CoreDNS{}}, gardencorev1beta1.CoreDNSAutoscalingModeHorizontal, true),
		Entry("with empty core dns autoscaling (cluster-proportional)", &gardencorev1beta1.SystemComponents{CoreDNS: &gardencorev1beta1.CoreDNS{Autoscaling: &gardencorev1beta1.CoreDNSAutoscaling{}}}, gardencorev1beta1.CoreDNSAutoscalingModeClusterProportional, false),
		Entry("with empty core dns autoscaling (horizontal)", &gardencorev1beta1.SystemComponents{CoreDNS: &gardencorev1beta1.CoreDNS{Autoscaling: &gardencorev1beta1.CoreDNSAutoscaling{}}}, gardencorev1beta1.CoreDNSAutoscalingModeHorizontal, false),
		Entry("with incorrect autoscaling mode (cluster-proportional)", &gardencorev1beta1.SystemComponents{CoreDNS: &gardencorev1beta1.CoreDNS{Autoscaling: &gardencorev1beta1.CoreDNSAutoscaling{Mode: "test"}}}, gardencorev1beta1.CoreDNSAutoscalingModeClusterProportional, false),
		Entry("with incorrect autoscaling mode (horizonal)", &gardencorev1beta1.SystemComponents{CoreDNS: &gardencorev1beta1.CoreDNS{Autoscaling: &gardencorev1beta1.CoreDNSAutoscaling{Mode: "test"}}}, gardencorev1beta1.CoreDNSAutoscalingModeHorizontal, false),
		Entry("with horizontal autoscaling mode (cluster-proportional)", &gardencorev1beta1.SystemComponents{CoreDNS: &gardencorev1beta1.CoreDNS{Autoscaling: &gardencorev1beta1.CoreDNSAutoscaling{Mode: "horizontal"}}}, gardencorev1beta1.CoreDNSAutoscalingModeClusterProportional, false),
		Entry("with horizontal autoscaling mode (horizontal)", &gardencorev1beta1.SystemComponents{CoreDNS: &gardencorev1beta1.CoreDNS{Autoscaling: &gardencorev1beta1.CoreDNSAutoscaling{Mode: "horizontal"}}}, gardencorev1beta1.CoreDNSAutoscalingModeHorizontal, true),
		Entry("with cluster-proportional autoscaling mode (cluster-proportional)", &gardencorev1beta1.SystemComponents{CoreDNS: &gardencorev1beta1.CoreDNS{Autoscaling: &gardencorev1beta1.CoreDNSAutoscaling{Mode: "cluster-proportional"}}}, gardencorev1beta1.CoreDNSAutoscalingModeClusterProportional, true),
		Entry("with cluster-proportional autoscaling mode (horizontal)", &gardencorev1beta1.SystemComponents{CoreDNS: &gardencorev1beta1.CoreDNS{Autoscaling: &gardencorev1beta1.CoreDNSAutoscaling{Mode: "cluster-proportional"}}}, gardencorev1beta1.CoreDNSAutoscalingModeHorizontal, false),
	)

	Context("#IsCoreDNSRewritingEnabled feature gate context", func() {
		BeforeEach(func() {
			gardenletfeatures.RegisterFeatureGates()
		})

		DescribeTable("#IsCoreDNSRewritingEnabled",
			func(featureGate bool, annotations map[string]string, expected bool) {
				Expect(IsCoreDNSRewritingEnabled(featureGate, annotations)).To(Equal(expected))
			},

			Entry("with feature gate enabled and no annotation", true, map[string]string{}, true),
			Entry("with feature gate disabled and no annotation", false, map[string]string{}, false),
			Entry("with feature gate enabled and incorrect annotations", true, map[string]string{"some annotation": "some value", "foo": "bar"}, true),
			Entry("with feature gate disabled and incorrect annotation", false, map[string]string{"some annotation": "some value", "foo": "bar"}, false),
			Entry("with feature gate enabled and correct annotations", true, map[string]string{v1beta1constants.AnnotationCoreDNSRewritingDisabled: "some value"}, false),
			Entry("with feature gate disabled and correct annotation", false, map[string]string{v1beta1constants.AnnotationCoreDNSRewritingDisabled: "some value"}, false),
		)
	})

	DescribeTable("#IsNodeLocalDNSEnabled",
		func(systemComponents *gardencorev1beta1.SystemComponents, annotations map[string]string, expected bool) {
			Expect(IsNodeLocalDNSEnabled(systemComponents, annotations)).To(Equal(expected))
		},

		Entry("with nil (disabled)", nil, nil, false),
		Entry("with empty system components and no proper annotation (disabled)", &gardencorev1beta1.SystemComponents{}, map[string]string{"something": "wrong"}, false),
		Entry("with system components and no proper annotation (disabled)", &gardencorev1beta1.SystemComponents{NodeLocalDNS: &gardencorev1beta1.NodeLocalDNS{}}, map[string]string{"something": "wrong"}, false),
		Entry("with system components and no proper annotation (enabled)", &gardencorev1beta1.SystemComponents{NodeLocalDNS: &gardencorev1beta1.NodeLocalDNS{Enabled: true}}, map[string]string{"something": "wrong"}, true),
		Entry("with system components and no proper annotation (disabled)", &gardencorev1beta1.SystemComponents{NodeLocalDNS: &gardencorev1beta1.NodeLocalDNS{Enabled: false}}, map[string]string{"something": "wrong"}, false),
		Entry("with empty system components and proper annotation (disabled)", &gardencorev1beta1.SystemComponents{}, map[string]string{v1beta1constants.AnnotationNodeLocalDNS: "false"}, false),
		Entry("with empty system components and proper annotation (enabled)", &gardencorev1beta1.SystemComponents{}, map[string]string{v1beta1constants.AnnotationNodeLocalDNS: "true"}, true),
		Entry("with empty system components and proper annotation, but wrong value (disabled)", &gardencorev1beta1.SystemComponents{}, map[string]string{v1beta1constants.AnnotationNodeLocalDNS: "test"}, false),
		Entry("with system components and proper annotation (enabled)", &gardencorev1beta1.SystemComponents{NodeLocalDNS: &gardencorev1beta1.NodeLocalDNS{Enabled: true}}, map[string]string{v1beta1constants.AnnotationNodeLocalDNS: "true"}, true),
		Entry("with system components and proper annotation (disabled)", &gardencorev1beta1.SystemComponents{NodeLocalDNS: &gardencorev1beta1.NodeLocalDNS{Enabled: false}}, map[string]string{v1beta1constants.AnnotationNodeLocalDNS: "false"}, false),
		Entry("with system components and proper annotation (enabled)", &gardencorev1beta1.SystemComponents{NodeLocalDNS: &gardencorev1beta1.NodeLocalDNS{Enabled: true}}, map[string]string{v1beta1constants.AnnotationNodeLocalDNS: "false"}, true),
		Entry("with system components and proper annotation (enabled)", &gardencorev1beta1.SystemComponents{NodeLocalDNS: &gardencorev1beta1.NodeLocalDNS{Enabled: false}}, map[string]string{v1beta1constants.AnnotationNodeLocalDNS: "true"}, true),
	)

	DescribeTable("#GetNodeLocalDNS",
		func(systemComponents *gardencorev1beta1.SystemComponents, expected *gardencorev1beta1.NodeLocalDNS) {
			Expect(GetNodeLocalDNS(systemComponents)).To(Equal(expected))
		},
		Entry("with nil", nil, nil),
		Entry("with system components and nil", nil, nil),
		Entry("with system components and node local DNS spec", &gardencorev1beta1.SystemComponents{NodeLocalDNS: &gardencorev1beta1.NodeLocalDNS{Enabled: true, ForceTCPToClusterDNS: pointer.Bool(true), ForceTCPToUpstreamDNS: pointer.Bool(true), DisableForwardToUpstreamDNS: pointer.Bool(true)}}, &gardencorev1beta1.NodeLocalDNS{Enabled: true, ForceTCPToClusterDNS: pointer.Bool(true), ForceTCPToUpstreamDNS: pointer.Bool(true), DisableForwardToUpstreamDNS: pointer.Bool(true)}),
	)

	DescribeTable("#GetShootCARotationPhase",
		func(credentials *gardencorev1beta1.ShootCredentials, expectedPhase gardencorev1beta1.CredentialsRotationPhase) {
			Expect(GetShootCARotationPhase(credentials)).To(Equal(expectedPhase))
		},

		Entry("credentials nil", nil, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("rotation nil", &gardencorev1beta1.ShootCredentials{}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("ca nil", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{}}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("phase empty", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{CertificateAuthorities: &gardencorev1beta1.CARotation{}}}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("phase set", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{CertificateAuthorities: &gardencorev1beta1.CARotation{Phase: gardencorev1beta1.RotationCompleting}}}, gardencorev1beta1.RotationCompleting),
	)

	Describe("#MutateShootCARotation", func() {
		It("should do nothing when mutate function is nil", func() {
			shoot := &gardencorev1beta1.Shoot{}
			MutateShootCARotation(shoot, nil)
			Expect(GetShootCARotationPhase(shoot.Status.Credentials)).To(BeEmpty())
		})

		DescribeTable("mutate function not nil",
			func(shoot *gardencorev1beta1.Shoot, phase gardencorev1beta1.CredentialsRotationPhase) {
				MutateShootCARotation(shoot, func(rotation *gardencorev1beta1.CARotation) {
					rotation.Phase = phase
				})
				Expect(shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(phase))
			},

			Entry("credentials nil", &gardencorev1beta1.Shoot{}, gardencorev1beta1.RotationCompleting),
			Entry("rotation nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{}}}, gardencorev1beta1.RotationCompleting),
			Entry("certificateAuthorities nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{}}}}, gardencorev1beta1.RotationCompleting),
			Entry("certificateAuthorities non-nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{CertificateAuthorities: &gardencorev1beta1.CARotation{}}}}}, gardencorev1beta1.RotationCompleting),
		)
	})

	Describe("#MutateShootKubeconfigRotation", func() {
		It("should do nothing when mutate function is nil", func() {
			shoot := &gardencorev1beta1.Shoot{}
			MutateShootKubeconfigRotation(shoot, nil)
			Expect(shoot.Status.Credentials).To(BeNil())
		})

		DescribeTable("mutate function not nil",
			func(shoot *gardencorev1beta1.Shoot, lastInitiationTime metav1.Time) {
				MutateShootKubeconfigRotation(shoot, func(rotation *gardencorev1beta1.ShootKubeconfigRotation) {
					rotation.LastInitiationTime = &lastInitiationTime
				})
				Expect(shoot.Status.Credentials.Rotation.Kubeconfig.LastInitiationTime).To(PointTo(Equal(lastInitiationTime)))
			},

			Entry("credentials nil", &gardencorev1beta1.Shoot{}, metav1.Now()),
			Entry("rotation nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{}}}, metav1.Now()),
			Entry("kubeconfig nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{}}}}, metav1.Now()),
			Entry("kubeconfig non-nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{Kubeconfig: &gardencorev1beta1.ShootKubeconfigRotation{}}}}}, metav1.Now()),
		)
	})

	DescribeTable("#IsShootKubeconfigRotationInitiationTimeAfterLastCompletionTime",
		func(credentials *gardencorev1beta1.ShootCredentials, matcher gomegatypes.GomegaMatcher) {
			Expect(IsShootKubeconfigRotationInitiationTimeAfterLastCompletionTime(credentials)).To(matcher)
		},

		Entry("credentials nil", nil, BeFalse()),
		Entry("rotation nil", &gardencorev1beta1.ShootCredentials{}, BeFalse()),
		Entry("kubeconfig nil", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{}}, BeFalse()),
		Entry("lastInitiationTime nil", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{Kubeconfig: &gardencorev1beta1.ShootKubeconfigRotation{}}}, BeFalse()),
		Entry("lastCompletionTime nil", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{Kubeconfig: &gardencorev1beta1.ShootKubeconfigRotation{LastInitiationTime: timePointer(metav1.Now().Time)}}}, BeTrue()),
		Entry("lastCompletionTime before lastInitiationTime", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{Kubeconfig: &gardencorev1beta1.ShootKubeconfigRotation{LastInitiationTime: timePointer(metav1.Now().Time), LastCompletionTime: timePointer(metav1.Now().Add(-time.Minute))}}}, BeTrue()),
		Entry("lastCompletionTime equal lastInitiationTime", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{Kubeconfig: &gardencorev1beta1.ShootKubeconfigRotation{LastInitiationTime: timePointer(metav1.Now().Time), LastCompletionTime: timePointer(metav1.Now().Time)}}}, BeFalse()),
		Entry("lastCompletionTime after lastInitiationTime", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{Kubeconfig: &gardencorev1beta1.ShootKubeconfigRotation{LastInitiationTime: timePointer(metav1.Now().Time), LastCompletionTime: timePointer(metav1.Now().Add(time.Minute))}}}, BeFalse()),
	)

	Describe("#MutateShootSSHKeypairRotation", func() {
		It("should do nothing when mutate function is nil", func() {
			shoot := &gardencorev1beta1.Shoot{}
			MutateShootSSHKeypairRotation(shoot, nil)
			Expect(shoot.Status.Credentials).To(BeNil())
		})

		DescribeTable("mutate function not nil",
			func(shoot *gardencorev1beta1.Shoot, lastInitiationTime metav1.Time) {
				MutateShootSSHKeypairRotation(shoot, func(rotation *gardencorev1beta1.ShootSSHKeypairRotation) {
					rotation.LastInitiationTime = &lastInitiationTime
				})
				Expect(shoot.Status.Credentials.Rotation.SSHKeypair.LastInitiationTime).To(PointTo(Equal(lastInitiationTime)))
			},

			Entry("credentials nil", &gardencorev1beta1.Shoot{}, metav1.Now()),
			Entry("rotation nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{}}}, metav1.Now()),
			Entry("sshKeypair nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{}}}}, metav1.Now()),
			Entry("sshKeypair non-nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{SSHKeypair: &gardencorev1beta1.ShootSSHKeypairRotation{}}}}}, metav1.Now()),
		)
	})

	DescribeTable("#IsShootSSHKeypairRotationInitiationTimeAfterLastCompletionTime",
		func(credentials *gardencorev1beta1.ShootCredentials, matcher gomegatypes.GomegaMatcher) {
			Expect(IsShootSSHKeypairRotationInitiationTimeAfterLastCompletionTime(credentials)).To(matcher)
		},

		Entry("credentials nil", nil, BeFalse()),
		Entry("rotation nil", &gardencorev1beta1.ShootCredentials{}, BeFalse()),
		Entry("sshKeypair nil", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{}}, BeFalse()),
		Entry("lastInitiationTime nil", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{SSHKeypair: &gardencorev1beta1.ShootSSHKeypairRotation{}}}, BeFalse()),
		Entry("lastCompletionTime nil", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{SSHKeypair: &gardencorev1beta1.ShootSSHKeypairRotation{LastInitiationTime: timePointer(metav1.Now().Time)}}}, BeTrue()),
		Entry("lastCompletionTime before lastInitiationTime", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{SSHKeypair: &gardencorev1beta1.ShootSSHKeypairRotation{LastInitiationTime: timePointer(metav1.Now().Time), LastCompletionTime: timePointer(metav1.Now().Add(-time.Minute))}}}, BeTrue()),
		Entry("lastCompletionTime equal lastInitiationTime", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{SSHKeypair: &gardencorev1beta1.ShootSSHKeypairRotation{LastInitiationTime: timePointer(metav1.Now().Time), LastCompletionTime: timePointer(metav1.Now().Time)}}}, BeFalse()),
		Entry("lastCompletionTime after lastInitiationTime", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{SSHKeypair: &gardencorev1beta1.ShootSSHKeypairRotation{LastInitiationTime: timePointer(metav1.Now().Time), LastCompletionTime: timePointer(metav1.Now().Add(time.Minute))}}}, BeFalse()),
	)

	Describe("#MutateObservabilityRotation", func() {
		It("should do nothing when mutate function is nil", func() {
			shoot := &gardencorev1beta1.Shoot{}
			MutateObservabilityRotation(shoot, nil)
			Expect(shoot.Status.Credentials).To(BeNil())
		})

		DescribeTable("mutate function not nil",
			func(shoot *gardencorev1beta1.Shoot, lastInitiationTime metav1.Time) {
				MutateObservabilityRotation(shoot, func(rotation *gardencorev1beta1.ShootObservabilityRotation) {
					rotation.LastInitiationTime = &lastInitiationTime
				})
				Expect(shoot.Status.Credentials.Rotation.Observability.LastInitiationTime).To(PointTo(Equal(lastInitiationTime)))
			},

			Entry("credentials nil", &gardencorev1beta1.Shoot{}, metav1.Now()),
			Entry("rotation nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{}}}, metav1.Now()),
			Entry("observability nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{}}}}, metav1.Now()),
			Entry("observability non-nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{Observability: &gardencorev1beta1.ShootObservabilityRotation{}}}}}, metav1.Now()),
		)
	})

	DescribeTable("#IsShootObservabilityRotationInitiationTimeAfterLastCompletionTime",
		func(credentials *gardencorev1beta1.ShootCredentials, matcher gomegatypes.GomegaMatcher) {
			Expect(IsShootObservabilityRotationInitiationTimeAfterLastCompletionTime(credentials)).To(matcher)
		},

		Entry("credentials nil", nil, BeFalse()),
		Entry("rotation nil", &gardencorev1beta1.ShootCredentials{}, BeFalse()),
		Entry("observability nil", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{}}, BeFalse()),
		Entry("lastInitiationTime nil", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{Observability: &gardencorev1beta1.ShootObservabilityRotation{}}}, BeFalse()),
		Entry("lastCompletionTime nil", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{Observability: &gardencorev1beta1.ShootObservabilityRotation{LastInitiationTime: timePointer(metav1.Now().Time)}}}, BeTrue()),
		Entry("lastCompletionTime before lastInitiationTime", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{Observability: &gardencorev1beta1.ShootObservabilityRotation{LastInitiationTime: timePointer(metav1.Now().Time), LastCompletionTime: timePointer(metav1.Now().Add(-time.Minute))}}}, BeTrue()),
		Entry("lastCompletionTime equal lastInitiationTime", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{Observability: &gardencorev1beta1.ShootObservabilityRotation{LastInitiationTime: timePointer(metav1.Now().Time), LastCompletionTime: timePointer(metav1.Now().Time)}}}, BeFalse()),
		Entry("lastCompletionTime after lastInitiationTime", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{Observability: &gardencorev1beta1.ShootObservabilityRotation{LastInitiationTime: timePointer(metav1.Now().Time), LastCompletionTime: timePointer(metav1.Now().Add(time.Minute))}}}, BeFalse()),
	)

	DescribeTable("#GetShootServiceAccountKeyRotationPhase",
		func(credentials *gardencorev1beta1.ShootCredentials, expectedPhase gardencorev1beta1.CredentialsRotationPhase) {
			Expect(GetShootServiceAccountKeyRotationPhase(credentials)).To(Equal(expectedPhase))
		},

		Entry("credentials nil", nil, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("rotation nil", &gardencorev1beta1.ShootCredentials{}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("serviceAccountKey nil", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{}}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("phase empty", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{}}}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("phase set", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{Phase: gardencorev1beta1.RotationCompleting}}}, gardencorev1beta1.RotationCompleting),
	)

	Describe("#MutateShootServiceAccountKeyRotation", func() {
		It("should do nothing when mutate function is nil", func() {
			shoot := &gardencorev1beta1.Shoot{}
			MutateShootServiceAccountKeyRotation(shoot, nil)
			Expect(GetShootServiceAccountKeyRotationPhase(shoot.Status.Credentials)).To(BeEmpty())
		})

		DescribeTable("mutate function not nil",
			func(shoot *gardencorev1beta1.Shoot, phase gardencorev1beta1.CredentialsRotationPhase) {
				MutateShootServiceAccountKeyRotation(shoot, func(rotation *gardencorev1beta1.ServiceAccountKeyRotation) {
					rotation.Phase = phase
				})
				Expect(shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase).To(Equal(phase))
			},

			Entry("credentials nil", &gardencorev1beta1.Shoot{}, gardencorev1beta1.RotationCompleting),
			Entry("rotation nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{}}}, gardencorev1beta1.RotationCompleting),
			Entry("serviceAccountKey nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{}}}}, gardencorev1beta1.RotationCompleting),
			Entry("serviceAccountKey non-nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{ServiceAccountKey: &gardencorev1beta1.ServiceAccountKeyRotation{}}}}}, gardencorev1beta1.RotationCompleting),
		)
	})

	DescribeTable("#GetShootETCDEncryptionKeyRotationPhase",
		func(credentials *gardencorev1beta1.ShootCredentials, expectedPhase gardencorev1beta1.CredentialsRotationPhase) {
			Expect(GetShootETCDEncryptionKeyRotationPhase(credentials)).To(Equal(expectedPhase))
		},

		Entry("credentials nil", nil, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("rotation nil", &gardencorev1beta1.ShootCredentials{}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("etcdEncryptionKey nil", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{}}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("phase empty", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{}}}, gardencorev1beta1.CredentialsRotationPhase("")),
		Entry("phase set", &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{Phase: gardencorev1beta1.RotationCompleting}}}, gardencorev1beta1.RotationCompleting),
	)

	Describe("#MutateShootETCDEncryptionKeyRotation", func() {
		It("should do nothing when mutate function is nil", func() {
			shoot := &gardencorev1beta1.Shoot{}
			MutateShootETCDEncryptionKeyRotation(shoot, nil)
			Expect(GetShootETCDEncryptionKeyRotationPhase(shoot.Status.Credentials)).To(BeEmpty())
		})

		DescribeTable("mutate function not nil",
			func(shoot *gardencorev1beta1.Shoot, phase gardencorev1beta1.CredentialsRotationPhase) {
				MutateShootETCDEncryptionKeyRotation(shoot, func(rotation *gardencorev1beta1.ETCDEncryptionKeyRotation) {
					rotation.Phase = phase
				})
				Expect(shoot.Status.Credentials.Rotation.ETCDEncryptionKey.Phase).To(Equal(phase))
			},

			Entry("credentials nil", &gardencorev1beta1.Shoot{}, gardencorev1beta1.RotationCompleting),
			Entry("rotation nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{}}}, gardencorev1beta1.RotationCompleting),
			Entry("etcdEncryptionKey nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{}}}}, gardencorev1beta1.RotationCompleting),
			Entry("etcdEncryptionKey non-nil", &gardencorev1beta1.Shoot{Status: gardencorev1beta1.ShootStatus{Credentials: &gardencorev1beta1.ShootCredentials{Rotation: &gardencorev1beta1.ShootCredentialsRotation{ETCDEncryptionKey: &gardencorev1beta1.ETCDEncryptionKeyRotation{}}}}}, gardencorev1beta1.RotationCompleting),
		)
	})

	Describe("#IsPSPDisabled", func() {
		var shoot *gardencorev1beta1.Shoot

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version:       "1.24.0",
						KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{},
					},
				},
			}
		})

		It("should return true if Kubernetes version >= 1.25", func() {
			shoot.Spec.Kubernetes.Version = "1.25.0"

			Expect(IsPSPDisabled(shoot)).To(BeTrue())
		})

		It("should return true if PodSecurityPolicy admissionPlugin is disabled", func() {
			shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{
				{
					Name:     "PodSecurityPolicy",
					Disabled: pointer.Bool(true),
				},
			}

			Expect(IsPSPDisabled(shoot)).To(BeTrue())
		})

		It("should return false if PodSecurityPolicy admissionPlugin is not disabled", func() {
			shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{
				{
					Name: "PodSecurityPolicy",
				},
			}

			Expect(IsPSPDisabled(shoot)).To(BeFalse())
		})

		It("should return false if PodSecurityPolicy admissionPlugin is not specified in the shootSpec", func() {
			shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []gardencorev1beta1.AdmissionPlugin{
				{
					Name: "NamespaceLifecycle",
				},
			}

			Expect(IsPSPDisabled(shoot)).To(BeFalse())
		})

		It("should return false if KubeAPIServerConfig is nil", func() {
			shoot.Spec.Kubernetes.KubeAPIServer = nil

			Expect(IsPSPDisabled(shoot)).To(BeFalse())
		})
	})

	DescribeTable("#IsFailureToleranceTypeZone",
		func(failureToleranceType *gardencorev1beta1.FailureToleranceType, expectedResult bool) {
			Expect(IsFailureToleranceTypeZone(failureToleranceType)).To(Equal(expectedResult))
		},

		Entry("failureToleranceType is zone", failureToleranceTypePointer(gardencorev1beta1.FailureToleranceTypeZone), true),
		Entry("failureToleranceType is node", failureToleranceTypePointer(gardencorev1beta1.FailureToleranceTypeNode), false),
		Entry("failureToleranceType is nil", nil, false),
	)

	DescribeTable("#IsFailureToleranceTypeNode",
		func(failureToleranceType *gardencorev1beta1.FailureToleranceType, expectedResult bool) {
			Expect(IsFailureToleranceTypeNode(failureToleranceType)).To(Equal(expectedResult))
		},

		Entry("failureToleranceType is zone", failureToleranceTypePointer(gardencorev1beta1.FailureToleranceTypeZone), false),
		Entry("failureToleranceType is node", failureToleranceTypePointer(gardencorev1beta1.FailureToleranceTypeNode), true),
		Entry("failureToleranceType is nil", nil, false),
	)

	Describe("#IsHAControlPlaneConfigured", func() {
		var shoot *gardencorev1beta1.Shoot

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{}
		})

		It("return false when HighAvailability is not set", func() {
			shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{}
			Expect(IsHAControlPlaneConfigured(shoot)).To(BeFalse())
		})

		It("return false when ControlPlane is not set", func() {
			Expect(IsHAControlPlaneConfigured(shoot)).To(BeFalse())
		})

		It("should return true when HighAvailability is set", func() {
			shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
				HighAvailability: &gardencorev1beta1.HighAvailability{},
			}
			Expect(IsHAControlPlaneConfigured(shoot)).To(BeTrue())
		})
	})

	Describe("#IsMultiZonalShootControlPlane", func() {
		var shoot *gardencorev1beta1.Shoot

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{}
		})

		It("should return false when shoot has no ControlPlane Spec", func() {
			Expect(IsMultiZonalShootControlPlane(shoot)).To(BeFalse())
		})

		It("should return false when shoot has no HighAvailability Spec", func() {
			shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{}
			Expect(IsMultiZonalShootControlPlane(shoot)).To(BeFalse())
		})

		It("should return false when shoot defines failure tolerance type 'node'", func() {
			shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeNode}}}
			Expect(IsMultiZonalShootControlPlane(shoot)).To(BeFalse())
		})

		It("should return true when shoot defines failure tolerance type 'zone'", func() {
			shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeZone}}}
			Expect(IsMultiZonalShootControlPlane(shoot)).To(BeTrue())
		})
	})

	Describe("#IsWorkerless", func() {
		var shoot *gardencorev1beta1.Shoot

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{
							{
								Name: "worker",
							},
						},
					},
				},
			}
		})

		It("should return false when shoot has workers", func() {
			Expect(IsWorkerless(shoot)).To(BeFalse())
		})

		It("should return true when shoot has zero workers", func() {
			shoot.Spec.Provider.Workers = nil
			Expect(IsWorkerless(shoot)).To(BeTrue())
		})
	})

	DescribeTable("#ShootEnablesSSHAccess",
		func(workersSettings *gardencorev1beta1.WorkersSettings, expectedResult bool) {
			shoot := &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						WorkersSettings: workersSettings,
					},
				},
			}
			Expect(ShootEnablesSSHAccess(shoot)).To(Equal(expectedResult))
		},

		Entry("should return true when shoot provider has no WorkersSettings", nil, true),
		Entry("should return true when shoot worker settings has no SSHAccess", &gardencorev1beta1.WorkersSettings{}, true),
		Entry("should return true when shoot worker settings has SSHAccess set to true", &gardencorev1beta1.WorkersSettings{SSHAccess: &gardencorev1beta1.SSHAccess{Enabled: true}}, true),
		Entry("should return false when shoot worker settings has SSHAccess set to false", &gardencorev1beta1.WorkersSettings{SSHAccess: &gardencorev1beta1.SSHAccess{Enabled: false}}, false),
	)

	Describe("#GetFailureToleranceType", func() {
		var shoot *gardencorev1beta1.Shoot

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{}
		})

		It("should return 'nil' when ControlPlane is empty", func() {
			shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{}
			Expect(GetFailureToleranceType(shoot)).To(BeNil())
		})

		It("should return type 'node' when set in ControlPlane.HighAvailability", func() {
			shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
				HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeNode}},
			}
			Expect(GetFailureToleranceType(shoot)).To(PointTo(Equal(gardencorev1beta1.FailureToleranceTypeNode)))
		})
	})

	DescribeTable("#SeedWantsManagedIngress",
		func(seed *gardencorev1beta1.Seed, expected gomegatypes.GomegaMatcher) {
			Expect(SeedWantsManagedIngress(seed)).To(expected)
		},

		Entry("dns provider nil", &gardencorev1beta1.Seed{}, BeFalse()),
		Entry("ingress nil", &gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{DNS: gardencorev1beta1.SeedDNS{Provider: &gardencorev1beta1.SeedDNSProvider{}}}}, BeFalse()),
		Entry("ingress controller kind not nginx", &gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{DNS: gardencorev1beta1.SeedDNS{Provider: &gardencorev1beta1.SeedDNSProvider{}}, Ingress: &gardencorev1beta1.Ingress{Controller: gardencorev1beta1.IngressController{Kind: "foo"}}}}, BeFalse()),
		Entry("ingress controller kind nginx", &gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{DNS: gardencorev1beta1.SeedDNS{Provider: &gardencorev1beta1.SeedDNSProvider{}}, Ingress: &gardencorev1beta1.Ingress{Controller: gardencorev1beta1.IngressController{Kind: "nginx"}}}}, BeTrue()),
	)

	DescribeTable("#IsTopologyAwareRoutingForShootControlPlaneEnabled",
		func(seed *gardencorev1beta1.Seed, shoot *gardencorev1beta1.Shoot, matcher gomegatypes.GomegaMatcher) {
			Expect(IsTopologyAwareRoutingForShootControlPlaneEnabled(seed, shoot)).To(matcher)
		},

		Entry("seed setting is nil, shoot control plane is not HA",
			&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{Settings: nil}},
			&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{ControlPlane: &gardencorev1beta1.ControlPlane{HighAvailability: nil}}},
			BeFalse(),
		),
		Entry("seed setting is disabled, shoot control plane is not HA",
			&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{Settings: &gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: false}}}},
			&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{ControlPlane: &gardencorev1beta1.ControlPlane{HighAvailability: nil}}},
			BeFalse(),
		),
		Entry("seed setting is enabled, shoot control plane is not HA",
			&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{Settings: &gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: true}}}},
			&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{ControlPlane: &gardencorev1beta1.ControlPlane{HighAvailability: nil}}},
			BeFalse(),
		),
		Entry("seed setting is nil, shoot control plane is HA with failure tolerance type 'zone'",
			&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{Settings: nil}},
			&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{ControlPlane: &gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeZone}}}}},
			BeFalse(),
		),
		Entry("seed setting is disabled, shoot control plane is HA with failure tolerance type 'zone'",
			&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{Settings: &gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: false}}}},
			&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{ControlPlane: &gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeZone}}}}},
			BeFalse(),
		),
		Entry("seed setting is enabled, shoot control plane is HA with failure tolerance type 'zone'",
			&gardencorev1beta1.Seed{Spec: gardencorev1beta1.SeedSpec{Settings: &gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: true}}}},
			&gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{ControlPlane: &gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeZone}}}}},
			BeTrue(),
		),
	)
})

func timePointer(t time.Time) *metav1.Time {
	return &metav1.Time{Time: t}
}

func failureToleranceTypePointer(failureToleranceType gardencorev1beta1.FailureToleranceType) *gardencorev1beta1.FailureToleranceType {
	return &failureToleranceType
}
