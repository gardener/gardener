// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	. "github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/operation/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	"time"
)

var (
	trueVar  = true
	falseVar = false
)

var _ = Describe("helper", func() {
	DescribeTable("#IsShootHibernated",
		func(shoot *gardenv1beta1.Shoot, hibernated bool) {
			Expect(IsShootHibernated(shoot)).To(Equal(hibernated))
		},
		Entry("no hibernation section", &gardenv1beta1.Shoot{}, false),
		Entry("hibernation.enabled = false", &gardenv1beta1.Shoot{
			Spec: gardenv1beta1.ShootSpec{
				Hibernation: &gardenv1beta1.Hibernation{Enabled: false},
			},
		}, false),
		Entry("hibernation.enabled = true", &gardenv1beta1.Shoot{
			Spec: gardenv1beta1.ShootSpec{
				Hibernation: &gardenv1beta1.Hibernation{Enabled: true},
			},
		}, true),
	)

	DescribeTable("#GetShootCloudProviderWOrkers",
		func(cloudProvider gardenv1beta1.CloudProvider, shoot *gardenv1beta1.Shoot, expected []gardenv1beta1.Worker) {
			Expect(GetShootCloudProviderWorkers(cloudProvider, shoot)).To(Equal(expected))
		},
		Entry("AWS",
			gardenv1beta1.CloudProviderAWS,
			&gardenv1beta1.Shoot{
				Spec: gardenv1beta1.ShootSpec{
					Cloud: gardenv1beta1.Cloud{
						AWS: &gardenv1beta1.AWSCloud{
							Workers: []gardenv1beta1.AWSWorker{{Worker: gardenv1beta1.Worker{Name: "aws"}}},
						},
					},
				},
			},
			[]gardenv1beta1.Worker{{Name: "aws"}}),
		Entry("Azure",
			gardenv1beta1.CloudProviderAzure,
			&gardenv1beta1.Shoot{
				Spec: gardenv1beta1.ShootSpec{
					Cloud: gardenv1beta1.Cloud{
						Azure: &gardenv1beta1.AzureCloud{
							Workers: []gardenv1beta1.AzureWorker{{Worker: gardenv1beta1.Worker{Name: "azure"}}},
						},
					},
				},
			},
			[]gardenv1beta1.Worker{{Name: "azure"}}),
		Entry("GCP",
			gardenv1beta1.CloudProviderGCP,
			&gardenv1beta1.Shoot{
				Spec: gardenv1beta1.ShootSpec{
					Cloud: gardenv1beta1.Cloud{
						GCP: &gardenv1beta1.GCPCloud{
							Workers: []gardenv1beta1.GCPWorker{{Worker: gardenv1beta1.Worker{Name: "gcp"}}},
						},
					},
				},
			},
			[]gardenv1beta1.Worker{{Name: "gcp"}}),
		Entry("OpenStack",
			gardenv1beta1.CloudProviderOpenStack,
			&gardenv1beta1.Shoot{
				Spec: gardenv1beta1.ShootSpec{
					Cloud: gardenv1beta1.Cloud{
						OpenStack: &gardenv1beta1.OpenStackCloud{
							Workers: []gardenv1beta1.OpenStackWorker{{Worker: gardenv1beta1.Worker{Name: "openStack"}}},
						},
					},
				},
			},
			[]gardenv1beta1.Worker{{Name: "openStack"}}),
		Entry("Local",
			gardenv1beta1.CloudProviderLocal,
			&gardenv1beta1.Shoot{},
			[]gardenv1beta1.Worker{{Name: "local", AutoScalerMin: 1, AutoScalerMax: 1}}))

	DescribeTable("#ShootWantsClusterAutoscaler",
		func(shoot *gardenv1beta1.Shoot, wantsAutoscaler bool) {
			actualWantsAutoscaler, err := ShootWantsClusterAutoscaler(shoot)

			Expect(err).NotTo(HaveOccurred())
			Expect(actualWantsAutoscaler).To(Equal(wantsAutoscaler))
		},
		Entry("no workers",
			&gardenv1beta1.Shoot{
				Spec: gardenv1beta1.ShootSpec{
					Cloud: gardenv1beta1.Cloud{
						OpenStack: &gardenv1beta1.OpenStackCloud{},
					},
				},
			},
			false),
		Entry("one worker no difference in auto scaler max and min",
			&gardenv1beta1.Shoot{
				Spec: gardenv1beta1.ShootSpec{
					Cloud: gardenv1beta1.Cloud{
						OpenStack: &gardenv1beta1.OpenStackCloud{
							Workers: []gardenv1beta1.OpenStackWorker{{Worker: gardenv1beta1.Worker{Name: "foo"}}},
						},
					},
				},
			},
			false),
		Entry("one worker with difference in auto scaler max and min",
			&gardenv1beta1.Shoot{
				Spec: gardenv1beta1.ShootSpec{
					Cloud: gardenv1beta1.Cloud{
						OpenStack: &gardenv1beta1.OpenStackCloud{
							Workers: []gardenv1beta1.OpenStackWorker{{Worker: gardenv1beta1.Worker{Name: "foo", AutoScalerMin: 1, AutoScalerMax: 2}}},
						},
					},
				},
			},
			true))

	DescribeTable("#UpdatedCondition",
		func(condition *gardenv1beta1.Condition, status corev1.ConditionStatus, reason, message string, matcher types.GomegaMatcher) {
			updated := UpdatedCondition(condition, status, reason, message)

			Expect(updated).To(matcher)
		},
		Entry("no update",
			&gardenv1beta1.Condition{
				Status:  corev1.ConditionTrue,
				Reason:  "reason",
				Message: "message",
			},
			corev1.ConditionTrue,
			"reason",
			"message",
			Equal(&gardenv1beta1.Condition{
				Status:  corev1.ConditionTrue,
				Reason:  "reason",
				Message: "message",
			}),
		),
		Entry("update",
			&gardenv1beta1.Condition{
				Status:  corev1.ConditionTrue,
				Reason:  "reason",
				Message: "message",
			},
			corev1.ConditionTrue,
			"OtherReason",
			"message",
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Status":             Equal(corev1.ConditionTrue),
				"Reason":             Equal("OtherReason"),
				"Message":            Equal("message"),
				"LastTransitionTime": Not(Equal(metav1.Time{Time: time.Unix(0, 0)})),
			})),
		),
	)

	Describe("#GetCondition", func() {
		It("should return the found condition", func() {
			var (
				conditionType gardenv1beta1.ConditionType = "test-1"
				condition                                 = gardenv1beta1.Condition{
					Type: conditionType,
				}
				conditions = []gardenv1beta1.Condition{condition}
			)

			cond := GetCondition(conditions, conditionType)

			Expect(cond).NotTo(BeNil())
			Expect(*cond).To(Equal(condition))
		})

		It("should return nil because the required condition could not be found", func() {
			var (
				conditionType gardenv1beta1.ConditionType = "test-1"
				conditions                                = []gardenv1beta1.Condition{}
			)

			cond := GetCondition(conditions, conditionType)

			Expect(cond).To(BeNil())
		})
	})

	Describe("#ReadShootedSeed", func() {
		var (
			shoot              *gardenv1beta1.Shoot
			defaultReplicas    int32 = 3
			defaultMinReplicas int32 = 3
			defaultMaxReplicas int32 = 3

			defaultAPIServerAutoscaler = ShootedSeedAPIServerAutoscaler{
				MinReplicas: &defaultMinReplicas,
				MaxReplicas: defaultMaxReplicas,
			}

			defaultAPIServer = ShootedSeedAPIServer{
				Replicas:   &defaultReplicas,
				Autoscaler: &defaultAPIServerAutoscaler,
			}

			defaultShootedSeed = ShootedSeed{
				APIServer: &defaultAPIServer,
			}
		)

		BeforeEach(func() {
			shoot = &gardenv1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   common.GardenNamespace,
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
				common.ShootUseAsSeed: "",
			}

			shootedSeed, err := ReadShootedSeed(shoot)

			Expect(err).NotTo(HaveOccurred())
			Expect(shootedSeed).To(BeNil())
		})

		It("should return true,nil,nil because annotation is set with normal usage", func() {
			shoot.Annotations = map[string]string{
				common.ShootUseAsSeed: "true",
			}

			shootedSeed, err := ReadShootedSeed(shoot)

			Expect(err).NotTo(HaveOccurred())
			Expect(shootedSeed).To(Equal(&defaultShootedSeed))
		})

		It("should return true,true,true because annotation is set with protected and visible usage", func() {
			shoot.Annotations = map[string]string{
				common.ShootUseAsSeed: "true,protected,visible",
			}

			shootedSeed, err := ReadShootedSeed(shoot)

			Expect(err).NotTo(HaveOccurred())
			Expect(shootedSeed).To(Equal(&ShootedSeed{
				Protected: &trueVar,
				Visible:   &trueVar,
				APIServer: &defaultAPIServer,
			}))
		})

		It("should return true,true,true because annotation is set with unprotected and invisible usage", func() {
			shoot.Annotations = map[string]string{
				common.ShootUseAsSeed: "true,unprotected,invisible",
			}

			shootedSeed, err := ReadShootedSeed(shoot)

			Expect(err).NotTo(HaveOccurred())
			Expect(shootedSeed).To(Equal(&ShootedSeed{
				Protected: &falseVar,
				Visible:   &falseVar,
				APIServer: &defaultAPIServer,
			}))
		})

		It("should return a filled apiserver config", func() {
			shoot.Annotations = map[string]string{
				common.ShootUseAsSeed: "true,apiServer.replicas=1,apiServer.autoscaler.minReplicas=2,apiServer.autoscaler.maxReplicas=3",
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
			}))
		})

		It("should fail due to maxReplicas not being specified", func() {
			shoot.Annotations = map[string]string{
				common.ShootUseAsSeed: "true,apiServer.autoscaler.minReplicas=2",
			}

			_, err := ReadShootedSeed(shoot)
			Expect(err).To(HaveOccurred())
		})

		It("should fail due to API server replicas being less than one", func() {
			shoot.Annotations = map[string]string{
				common.ShootUseAsSeed: "true,apiServer.replicas=0",
			}

			_, err := ReadShootedSeed(shoot)
			Expect(err).To(HaveOccurred())
		})

		It("should fail due to API server autoscaler minReplicas being less than one", func() {
			shoot.Annotations = map[string]string{
				common.ShootUseAsSeed: "true,apiServer.autoscaler.minReplicas=0,apiServer.autoscaler.maxReplicas=1",
			}

			_, err := ReadShootedSeed(shoot)
			Expect(err).To(HaveOccurred())
		})

		It("should fail due to API server autoscaler maxReplicas being less than one", func() {
			shoot.Annotations = map[string]string{
				common.ShootUseAsSeed: "true,apiServer.autoscaler.maxReplicas=0",
			}

			_, err := ReadShootedSeed(shoot)
			Expect(err).To(HaveOccurred())
		})

		It("should fail due to API server autoscaler minReplicas being greater than maxReplicas", func() {
			shoot.Annotations = map[string]string{
				common.ShootUseAsSeed: "true,apiServer.autoscaler.maxReplicas=1,apiServer.autoscaler.minReplicas=2",
			}

			_, err := ReadShootedSeed(shoot)
			Expect(err).To(HaveOccurred())
		})
	})
})
