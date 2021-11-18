// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"

	"github.com/Masterminds/semver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	gomegatypes "github.com/onsi/gomega/types"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
)

var _ = Describe("helper", func() {
	var (
		trueVar                 = true
		falseVar                = false
		expirationDateInThePast = metav1.Time{Time: time.Now().AddDate(0, 0, -1)}
	)

	Describe("errors", func() {
		var (
			testTime      = metav1.NewTime(time.Unix(10, 10))
			zeroTime      metav1.Time
			afterTestTime = func(t metav1.Time) bool { return t.After(testTime.Time) }
		)

		DescribeTable("#UpdatedCondition",
			func(condition gardencorev1beta1.Condition, status gardencorev1beta1.ConditionStatus, reason, message string, codes []gardencorev1beta1.ErrorCode, matcher types.GomegaMatcher) {
				updated := UpdatedCondition(condition, status, reason, message, codes...)

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

		Describe("#GetOrInitCondition", func() {
			It("should get the existing condition", func() {
				var (
					c          = gardencorev1beta1.Condition{Type: "foo"}
					conditions = []gardencorev1beta1.Condition{c}
				)

				Expect(GetOrInitCondition(conditions, "foo")).To(Equal(c))
			})

			It("should return a new, initialized condition", func() {
				tmp := Now
				Now = func() metav1.Time {
					return metav1.NewTime(time.Unix(0, 0))
				}
				defer func() { Now = tmp }()

				Expect(GetOrInitCondition(nil, "foo")).To(Equal(InitCondition("foo")))
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
						Type:   gardencorev1beta1.ControllerInstallationInstalled,
						Status: gardencorev1beta1.ConditionTrue,
					},
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
						Type:   gardencorev1beta1.ControllerInstallationInstalled,
						Status: gardencorev1beta1.ConditionFalse,
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationHealthy,
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
				Expect(HasOperationAnnotation(objectMeta)).To(Equal(expected))
			},
			Entry("reconcile", metav1.ObjectMeta{Annotations: map[string]string{v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile}}, true),
			Entry("restore", metav1.ObjectMeta{Annotations: map[string]string{v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationRestore}}, true),
			Entry("migrate", metav1.ObjectMeta{Annotations: map[string]string{v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationMigrate}}, true),
			Entry("unknown", metav1.ObjectMeta{Annotations: map[string]string{v1beta1constants.GardenerOperation: "unknown"}}, false),
			Entry("not present", metav1.ObjectMeta{}, false),
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

		Describe("#ReadShootedSeed", func() {
			var (
				shoot                    *gardencorev1beta1.Shoot
				defaultReplicas          int32 = 3
				defaultMinReplicas       int32 = 3
				defaultMaxReplicas       int32 = 3
				defaultMinimumVolumeSize       = "20Gi"

				defaultAPIServerAutoscaler = ShootedSeedAPIServerAutoscaler{
					MinReplicas: &defaultMinReplicas,
					MaxReplicas: defaultMaxReplicas,
				}

				defaultAPIServer = ShootedSeedAPIServer{
					Replicas:   &defaultReplicas,
					Autoscaler: &defaultAPIServerAutoscaler,
				}

				defaultResources = ShootedSeedResources{
					Capacity: corev1.ResourceList{
						gardencorev1beta1.ResourceShoots: resource.MustParse("250"),
					},
				}

				defaultShootedSeed = ShootedSeed{
					APIServer: &defaultAPIServer,
					Backup:    &gardencorev1beta1.SeedBackup{},
					Resources: &defaultResources,
				}
			)

			BeforeEach(func() {
				shoot = &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:   v1beta1constants.GardenNamespace,
						Annotations: nil,
					},
				}
			})

			It("should return false,nil,nil because shoot is not in the garden namespace", func() {
				shoot.Namespace = "default"

				shootedSeed, err := ReadShootedSeed(shoot)

				Expect(err).NotTo(HaveOccurred())
				Expect(shootedSeed).To(BeNil())
			})

			It("should return false,nil,nil because annotation is not set", func() {
				shootedSeed, err := ReadShootedSeed(shoot)

				Expect(err).NotTo(HaveOccurred())
				Expect(shootedSeed).To(BeNil())
			})

			It("should return false,nil,nil because annotation is set with no usages", func() {
				shoot.Annotations = map[string]string{
					v1beta1constants.AnnotationShootUseAsSeed: "",
				}

				shootedSeed, err := ReadShootedSeed(shoot)

				Expect(err).NotTo(HaveOccurred())
				Expect(shootedSeed).To(BeNil())
			})

			It("should return true,nil,nil because annotation is set with normal usage", func() {
				shoot.Annotations = map[string]string{
					v1beta1constants.AnnotationShootUseAsSeed: "true",
				}

				shootedSeed, err := ReadShootedSeed(shoot)

				Expect(err).NotTo(HaveOccurred())
				Expect(shootedSeed).To(Equal(&defaultShootedSeed))
			})

			It("should return true,true,true because annotation is set with protected and visible usage", func() {
				shoot.Annotations = map[string]string{
					v1beta1constants.AnnotationShootUseAsSeed: "true,protected,visible",
				}

				shootedSeed, err := ReadShootedSeed(shoot)

				Expect(err).NotTo(HaveOccurred())
				Expect(shootedSeed).To(Equal(&ShootedSeed{
					Protected: &trueVar,
					Visible:   &trueVar,
					APIServer: &defaultAPIServer,
					Backup:    &gardencorev1beta1.SeedBackup{},
					Resources: &defaultResources,
				}))
			})

			It("should return true,true,true because annotation is set with unprotected and invisible usage", func() {
				shoot.Annotations = map[string]string{
					v1beta1constants.AnnotationShootUseAsSeed: "true,unprotected,invisible",
				}

				shootedSeed, err := ReadShootedSeed(shoot)

				Expect(err).NotTo(HaveOccurred())
				Expect(shootedSeed).To(Equal(&ShootedSeed{
					Protected:         &falseVar,
					Visible:           &falseVar,
					APIServer:         &defaultAPIServer,
					Backup:            &gardencorev1beta1.SeedBackup{},
					MinimumVolumeSize: nil,
					Resources:         &defaultResources,
				}))
			})

			It("should return the min volume size because annotation is set properly", func() {
				shoot.Annotations = map[string]string{
					v1beta1constants.AnnotationShootUseAsSeed: "true,unprotected,invisible,minimumVolumeSize=20Gi",
				}

				shootedSeed, err := ReadShootedSeed(shoot)

				Expect(err).NotTo(HaveOccurred())
				Expect(shootedSeed).To(Equal(&ShootedSeed{
					Protected:         &falseVar,
					Visible:           &falseVar,
					APIServer:         &defaultAPIServer,
					Backup:            &gardencorev1beta1.SeedBackup{},
					MinimumVolumeSize: &defaultMinimumVolumeSize,
					Resources:         &defaultResources,
				}))
			})

			It("should return a filled apiserver config", func() {
				shoot.Annotations = map[string]string{
					v1beta1constants.AnnotationShootUseAsSeed: "true,apiServer.replicas=1,apiServer.autoscaler.minReplicas=2,apiServer.autoscaler.maxReplicas=3",
				}

				shootedSeed, err := ReadShootedSeed(shoot)

				var (
					one   int32 = 1
					two   int32 = 2
					three int32 = 3
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(shootedSeed).To(Equal(&ShootedSeed{
					APIServer: &ShootedSeedAPIServer{
						Replicas: &one,
						Autoscaler: &ShootedSeedAPIServerAutoscaler{
							MinReplicas: &two,
							MaxReplicas: three,
						},
					},
					Backup:    &gardencorev1beta1.SeedBackup{},
					Resources: &defaultResources,
				}))
			})

			It("should return a filled seedprovider providerconfig", func() {
				shoot.Annotations = map[string]string{
					v1beta1constants.AnnotationShootUseAsSeed: "true,providerConfig.storagePolicyName=vSAN Default Storage Policy,providerConfig.param1=abc" +
						",providerConfig.sub.param2=def,providerConfig.sub.param3=3,providerConfig.sub.param4=true,providerConfig.sub.param5=\"true\"",
				}

				shootedSeed, err := ReadShootedSeed(shoot)

				Expect(err).NotTo(HaveOccurred())
				Expect(shootedSeed).To(Equal(&ShootedSeed{
					APIServer: &defaultAPIServer,
					SeedProviderConfig: &runtime.RawExtension{
						Raw: []byte(`{"param1":"abc","storagePolicyName":"vSAN Default Storage Policy","sub":{"param2":"def","param3":3,"param4":true,"param5":"true"}}`),
					},
					Backup:    &gardencorev1beta1.SeedBackup{},
					Resources: &defaultResources,
				}))
			})

			It("should return a filled load balancer services annotations map", func() {
				shoot.Annotations = map[string]string{
					v1beta1constants.AnnotationShootUseAsSeed: "true,loadBalancerServices.annotations.role=apiserver,loadBalancerServices.annotations.service.beta.kubernetes.io/aws-load-balancer-type=nlb",
				}

				shootedSeed, err := ReadShootedSeed(shoot)

				Expect(err).NotTo(HaveOccurred())
				Expect(shootedSeed).To(Equal(&ShootedSeed{
					APIServer: &defaultAPIServer,
					LoadBalancerServicesAnnotations: map[string]string{
						"role": "apiserver",
						"service.beta.kubernetes.io/aws-load-balancer-type": "nlb",
					},
					Backup:    &gardencorev1beta1.SeedBackup{},
					Resources: &defaultResources,
				}))
			})

			It("should return a filled feature gates map", func() {
				shoot.Annotations = map[string]string{
					v1beta1constants.AnnotationShootUseAsSeed: "true,featureGates.Foo=bar,featureGates.Bar=true,featureGates.Baz=false",
				}

				shootedSeed, err := ReadShootedSeed(shoot)

				Expect(err).NotTo(HaveOccurred())
				Expect(shootedSeed).To(Equal(&ShootedSeed{
					APIServer: &defaultAPIServer,
					FeatureGates: map[string]bool{
						"Foo": false,
						"Bar": true,
						"Baz": false,
					},
					Backup:    &gardencorev1beta1.SeedBackup{},
					Resources: &defaultResources,
				}))
			})

			It("should return a filled resources", func() {
				shoot.Annotations = map[string]string{
					v1beta1constants.AnnotationShootUseAsSeed: "true,resources.capacity.shoots=150,resources.reserved.shoots=2",
				}

				shootedSeed, err := ReadShootedSeed(shoot)

				Expect(err).NotTo(HaveOccurred())
				Expect(shootedSeed).To(Equal(&ShootedSeed{
					APIServer: &defaultAPIServer,
					Backup:    &gardencorev1beta1.SeedBackup{},
					Resources: &ShootedSeedResources{
						Capacity: corev1.ResourceList{
							gardencorev1beta1.ResourceShoots: resource.MustParse("150"),
						},
						Reserved: corev1.ResourceList{
							gardencorev1beta1.ResourceShoots: resource.MustParse("2"),
						},
					},
				}))
			})

			It("should return a filled ingress controller providerconfig", func() {
				shoot.Annotations = map[string]string{
					v1beta1constants.AnnotationShootUseAsSeed: "true,ingress.controller.kind=foobar,ingress.controller.providerConfig.use-proxy-protocol=\"true\"," +
						"ingress.controller.providerConfig.server-name-hash-bucket-size=\"257\"",
				}

				shootedSeed, err := ReadShootedSeed(shoot)

				Expect(err).NotTo(HaveOccurred())
				Expect(shootedSeed).To(Equal(&ShootedSeed{
					APIServer: &defaultAPIServer,
					IngressController: &gardencorev1beta1.IngressController{
						Kind: "foobar",
						ProviderConfig: &runtime.RawExtension{
							Raw: []byte(`{"server-name-hash-bucket-size":"257","use-proxy-protocol":"true"}`),
						},
					},
					Backup:    &gardencorev1beta1.SeedBackup{},
					Resources: &defaultResources,
				}))
			})

			It("should fail due to maxReplicas not being specified", func() {
				shoot.Annotations = map[string]string{
					v1beta1constants.AnnotationShootUseAsSeed: "true,apiServer.autoscaler.minReplicas=2",
				}

				_, err := ReadShootedSeed(shoot)
				Expect(err).To(HaveOccurred())
			})

			It("should fail due to API server replicas being less than one", func() {
				shoot.Annotations = map[string]string{
					v1beta1constants.AnnotationShootUseAsSeed: "true,apiServer.replicas=0",
				}

				_, err := ReadShootedSeed(shoot)
				Expect(err).To(HaveOccurred())
			})

			It("should fail due to API server autoscaler minReplicas being less than one", func() {
				shoot.Annotations = map[string]string{
					v1beta1constants.AnnotationShootUseAsSeed: "true,apiServer.autoscaler.minReplicas=0,apiServer.autoscaler.maxReplicas=1",
				}

				_, err := ReadShootedSeed(shoot)
				Expect(err).To(HaveOccurred())
			})

			It("should fail due to API server autoscaler maxReplicas being less than one", func() {
				shoot.Annotations = map[string]string{
					v1beta1constants.AnnotationShootUseAsSeed: "true,apiServer.autoscaler.maxReplicas=0",
				}

				_, err := ReadShootedSeed(shoot)
				Expect(err).To(HaveOccurred())
			})

			It("should fail due to API server autoscaler minReplicas being greater than maxReplicas", func() {
				shoot.Annotations = map[string]string{
					v1beta1constants.AnnotationShootUseAsSeed: "true,apiServer.autoscaler.maxReplicas=1,apiServer.autoscaler.minReplicas=2",
				}

				_, err := ReadShootedSeed(shoot)
				Expect(err).To(HaveOccurred())
			})

			It("should fail due to the reserved value for a resource being greater than its capacity", func() {
				shoot.Annotations = map[string]string{
					v1beta1constants.AnnotationShootUseAsSeed: "true,resources.capacity.foo=42,resources.reserved.foo=43",
				}

				_, err := ReadShootedSeed(shoot)
				Expect(err).To(HaveOccurred())
			})

			It("should fail due to the reserved value for a resource not having capacity", func() {
				shoot.Annotations = map[string]string{
					v1beta1constants.AnnotationShootUseAsSeed: "true,resources.reserved.foo=42",
				}

				_, err := ReadShootedSeed(shoot)
				Expect(err).To(HaveOccurred())
			})
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
		func(providers []gardencorev1beta1.DNSProvider, matcher types.GomegaMatcher) {
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

	DescribeTable("#ShootAuditPolicyConfigMapRefEqual",
		func(oldAPIServerConfig, newAPIServerConfig *gardencorev1beta1.KubeAPIServerConfig, matcher gomegatypes.GomegaMatcher) {
			Expect(ShootAuditPolicyConfigMapRefEqual(oldAPIServerConfig, newAPIServerConfig)).To(matcher)
		},

		Entry("both nil", nil, nil, BeTrue()),
		Entry("old auditconfig nil", &gardencorev1beta1.KubeAPIServerConfig{}, &gardencorev1beta1.KubeAPIServerConfig{AuditConfig: &gardencorev1beta1.AuditConfig{AuditPolicy: &gardencorev1beta1.AuditPolicy{ConfigMapRef: &corev1.ObjectReference{Name: "foo"}}}}, BeFalse()),
		Entry("old auditpolicy nil", &gardencorev1beta1.KubeAPIServerConfig{AuditConfig: &gardencorev1beta1.AuditConfig{}}, &gardencorev1beta1.KubeAPIServerConfig{AuditConfig: &gardencorev1beta1.AuditConfig{AuditPolicy: &gardencorev1beta1.AuditPolicy{ConfigMapRef: &corev1.ObjectReference{Name: "foo"}}}}, BeFalse()),
		Entry("old configmapref nil", &gardencorev1beta1.KubeAPIServerConfig{AuditConfig: &gardencorev1beta1.AuditConfig{AuditPolicy: &gardencorev1beta1.AuditPolicy{}}}, &gardencorev1beta1.KubeAPIServerConfig{AuditConfig: &gardencorev1beta1.AuditConfig{AuditPolicy: &gardencorev1beta1.AuditPolicy{ConfigMapRef: &corev1.ObjectReference{Name: "foo"}}}}, BeFalse()),
		Entry("new auditconfig nil", &gardencorev1beta1.KubeAPIServerConfig{AuditConfig: &gardencorev1beta1.AuditConfig{AuditPolicy: &gardencorev1beta1.AuditPolicy{ConfigMapRef: &corev1.ObjectReference{Name: "foo"}}}}, &gardencorev1beta1.KubeAPIServerConfig{}, BeFalse()),
		Entry("new auditpolicy nil", &gardencorev1beta1.KubeAPIServerConfig{AuditConfig: &gardencorev1beta1.AuditConfig{AuditPolicy: &gardencorev1beta1.AuditPolicy{ConfigMapRef: &corev1.ObjectReference{Name: "foo"}}}}, &gardencorev1beta1.KubeAPIServerConfig{AuditConfig: &gardencorev1beta1.AuditConfig{}}, BeFalse()),
		Entry("new configmapref nil", &gardencorev1beta1.KubeAPIServerConfig{AuditConfig: &gardencorev1beta1.AuditConfig{AuditPolicy: &gardencorev1beta1.AuditPolicy{ConfigMapRef: &corev1.ObjectReference{Name: "foo"}}}}, &gardencorev1beta1.KubeAPIServerConfig{AuditConfig: &gardencorev1beta1.AuditConfig{AuditPolicy: &gardencorev1beta1.AuditPolicy{}}}, BeFalse()),
		Entry("difference", &gardencorev1beta1.KubeAPIServerConfig{AuditConfig: &gardencorev1beta1.AuditConfig{AuditPolicy: &gardencorev1beta1.AuditPolicy{ConfigMapRef: &corev1.ObjectReference{Name: "bar"}}}}, &gardencorev1beta1.KubeAPIServerConfig{AuditConfig: &gardencorev1beta1.AuditConfig{AuditPolicy: &gardencorev1beta1.AuditPolicy{ConfigMapRef: &corev1.ObjectReference{Name: "foo"}}}}, BeFalse()),
		Entry("equality", &gardencorev1beta1.KubeAPIServerConfig{AuditConfig: &gardencorev1beta1.AuditConfig{AuditPolicy: &gardencorev1beta1.AuditPolicy{ConfigMapRef: &corev1.ObjectReference{Name: "bar"}}}}, &gardencorev1beta1.KubeAPIServerConfig{AuditConfig: &gardencorev1beta1.AuditConfig{AuditPolicy: &gardencorev1beta1.AuditPolicy{ConfigMapRef: &corev1.ObjectReference{Name: "bar"}}}}, BeTrue()),
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

	DescribeTable("#ShootWantsAnonymousAuthentication",
		func(kubeAPIServerConfig *gardencorev1beta1.KubeAPIServerConfig, wantsAnonymousAuth bool) {
			actualWantsAnonymousAuth := ShootWantsAnonymousAuthentication(kubeAPIServerConfig)
			Expect(actualWantsAnonymousAuth).To(Equal(wantsAnonymousAuth))
		},

		Entry("no kubeapiserver configuration", nil, false),
		Entry("field not set", &gardencorev1beta1.KubeAPIServerConfig{}, false),
		Entry("explicitly enabled", &gardencorev1beta1.KubeAPIServerConfig{EnableAnonymousAuthentication: &trueVar}, true),
		Entry("explicitly disabled", &gardencorev1beta1.KubeAPIServerConfig{EnableAnonymousAuthentication: &falseVar}, false),
	)

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
})
