// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v1alpha1_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/operator/apis/config/v1alpha1"
)

var _ = Describe("Defaults", func() {
	Describe("OperatorConfiguration", func() {
		var obj *OperatorConfiguration

		BeforeEach(func() {
			obj = &OperatorConfiguration{}
		})

		It("should correctly default the configuration", func() {
			SetObjectDefaults_OperatorConfiguration(obj)

			Expect(obj.LogLevel).To(Equal(logger.InfoLevel))
			Expect(obj.LogFormat).To(Equal(logger.FormatJSON))

			Expect(obj.Server.Webhooks.BindAddress).To(BeEmpty())
			Expect(obj.Server.Webhooks.Port).To(Equal(2750))
			Expect(obj.Server.HealthProbes.BindAddress).To(BeEmpty())
			Expect(obj.Server.HealthProbes.Port).To(Equal(2751))
			Expect(obj.Server.Metrics.BindAddress).To(BeEmpty())
			Expect(obj.Server.Metrics.Port).To(Equal(2752))
		})

		It("should not overwrite custom settings", func() {
			var (
				expectedLogLevel  = "foo"
				expectedLogFormat = "bar"
				expectedServer    = ServerConfiguration{
					Webhooks: Server{
						BindAddress: "bay",
						Port:        3,
					},
					HealthProbes: &Server{
						BindAddress: "baz",
						Port:        1,
					},
					Metrics: &Server{
						BindAddress: "bax",
						Port:        2,
					},
				}
			)

			obj.LogLevel = expectedLogLevel
			obj.LogFormat = expectedLogFormat
			obj.Server = expectedServer

			SetObjectDefaults_OperatorConfiguration(obj)

			Expect(obj.LogLevel).To(Equal(expectedLogLevel))
			Expect(obj.LogFormat).To(Equal(expectedLogFormat))
			Expect(obj.Server).To(Equal(expectedServer))
		})

		Describe("RuntimeClientConnection", func() {
			It("should not default ContentType and AcceptContentTypes", func() {
				SetObjectDefaults_OperatorConfiguration(obj)

				// ContentType fields will be defaulted by client constructors / controller-runtime based on whether a
				// given APIGroup supports protobuf or not. defaults must not touch these, otherwise the integelligent
				// logic will be overwritten
				Expect(obj.RuntimeClientConnection.ContentType).To(BeEmpty())
				Expect(obj.RuntimeClientConnection.AcceptContentTypes).To(BeEmpty())
			})

			It("should correctly default RuntimeClientConnection", func() {
				SetObjectDefaults_OperatorConfiguration(obj)

				Expect(obj.RuntimeClientConnection).To(Equal(componentbaseconfigv1alpha1.ClientConnectionConfiguration{
					QPS:   100.0,
					Burst: 130,
				}))
			})
		})

		Describe("leader election settings", func() {
			It("should correctly default leader election settings", func() {
				SetObjectDefaults_OperatorConfiguration(obj)

				Expect(obj.LeaderElection).NotTo(BeNil())
				Expect(obj.LeaderElection.LeaderElect).To(PointTo(BeTrue()))
				Expect(obj.LeaderElection.LeaseDuration).To(Equal(metav1.Duration{Duration: 15 * time.Second}))
				Expect(obj.LeaderElection.RenewDeadline).To(Equal(metav1.Duration{Duration: 10 * time.Second}))
				Expect(obj.LeaderElection.RetryPeriod).To(Equal(metav1.Duration{Duration: 2 * time.Second}))
				Expect(obj.LeaderElection.ResourceLock).To(Equal("leases"))
				Expect(obj.LeaderElection.ResourceNamespace).To(Equal("garden"))
				Expect(obj.LeaderElection.ResourceName).To(Equal("gardener-operator-leader-election"))
			})

			It("should not overwrite custom settings", func() {
				expectedLeaderElection := componentbaseconfigv1alpha1.LeaderElectionConfiguration{
					LeaderElect:       pointer.Bool(true),
					ResourceLock:      "foo",
					RetryPeriod:       metav1.Duration{Duration: 40 * time.Second},
					RenewDeadline:     metav1.Duration{Duration: 41 * time.Second},
					LeaseDuration:     metav1.Duration{Duration: 42 * time.Second},
					ResourceNamespace: "other-garden-ns",
					ResourceName:      "lock-object",
				}
				obj.LeaderElection = expectedLeaderElection
				SetObjectDefaults_OperatorConfiguration(obj)

				Expect(obj.LeaderElection).To(Equal(expectedLeaderElection))
			})
		})

		Describe("Controller configuration", func() {
			Describe("Garden controller", func() {
				It("should not default the object", func() {
					obj := &GardenControllerConfig{}

					SetDefaults_GardenControllerConfig(obj)

					Expect(obj.ConcurrentSyncs).To(PointTo(Equal(1)))
					Expect(obj.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Hour})))
				})

				It("should not overwrite existing values", func() {
					obj := &GardenControllerConfig{
						ConcurrentSyncs: pointer.Int(5),
						SyncPeriod:      &metav1.Duration{Duration: time.Second},
					}

					SetDefaults_GardenControllerConfig(obj)

					Expect(obj.ConcurrentSyncs).To(PointTo(Equal(5)))
					Expect(obj.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Second})))
				})

				It("should correctly default ETCDConfig configuration", func() {
					SetDefaults_OperatorConfiguration(obj)
					Expect(obj.Controllers.Garden.ETCDConfig).NotTo(BeNil())
					Expect(obj.Controllers.Garden.ETCDConfig.ETCDController).NotTo(BeNil())
					Expect(obj.Controllers.Garden.ETCDConfig.ETCDController.Workers).To(PointTo(Equal(int64(50))))
					Expect(obj.Controllers.Garden.ETCDConfig.CustodianController).NotTo(BeNil())
					Expect(obj.Controllers.Garden.ETCDConfig.CustodianController.Workers).To(PointTo(Equal(int64(10))))
					Expect(obj.Controllers.Garden.ETCDConfig.BackupCompactionController).NotTo(BeNil())
					Expect(obj.Controllers.Garden.ETCDConfig.BackupCompactionController.Workers).To(PointTo(Equal(int64(3))))
					Expect(obj.Controllers.Garden.ETCDConfig.BackupCompactionController.EnableBackupCompaction).To(PointTo(Equal(false)))
					Expect(obj.Controllers.Garden.ETCDConfig.BackupCompactionController.EventsThreshold).To(PointTo(Equal(int64(1000000))))
				})
			})
		})
	})
})
