// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	. "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
)

var _ = Describe("Defaults", func() {
	Describe("ControllerManagerConfiguration", func() {
		var obj *ControllerManagerConfiguration

		BeforeEach(func() {
			obj = &ControllerManagerConfiguration{}
		})

		It("should correctly default the controller manager configuration", func() {
			SetObjectDefaults_ControllerManagerConfiguration(obj)

			Expect(obj.Controllers.Bastion).NotTo(BeNil())
			Expect(obj.Controllers.Bastion.ConcurrentSyncs).NotTo(BeNil())
			Expect(obj.Controllers.Bastion.ConcurrentSyncs).To(PointTo(Equal(5)))
			Expect(obj.Controllers.Bastion.MaxLifetime).To(PointTo(Equal(metav1.Duration{Duration: 24 * time.Hour})))

			Expect(obj.Controllers.CertificateSigningRequest).NotTo(BeNil())

			Expect(obj.Controllers.CloudProfile).NotTo(BeNil())
			Expect(obj.Controllers.CloudProfile.ConcurrentSyncs).NotTo(BeNil())
			Expect(obj.Controllers.CloudProfile.ConcurrentSyncs).To(PointTo(Equal(5)))

			Expect(obj.Controllers.ControllerDeployment).NotTo(BeNil())
			Expect(obj.Controllers.ControllerDeployment.ConcurrentSyncs).NotTo(BeNil())
			Expect(obj.Controllers.ControllerDeployment.ConcurrentSyncs).To(PointTo(Equal(5)))

			Expect(obj.Controllers.ControllerRegistration).NotTo(BeNil())
			Expect(obj.Controllers.ControllerRegistration.ConcurrentSyncs).NotTo(BeNil())
			Expect(obj.Controllers.ControllerRegistration.ConcurrentSyncs).To(PointTo(Equal(5)))

			Expect(obj.Controllers.ExposureClass).NotTo(BeNil())
			Expect(obj.Controllers.ExposureClass.ConcurrentSyncs).NotTo(BeNil())
			Expect(obj.Controllers.ExposureClass.ConcurrentSyncs).To(PointTo(Equal(5)))

			Expect(obj.Controllers.Project).NotTo(BeNil())
			Expect(obj.Controllers.Project.ConcurrentSyncs).NotTo(BeNil())
			Expect(obj.Controllers.Project.ConcurrentSyncs).To(PointTo(Equal(5)))
			Expect(obj.Controllers.Project.MinimumLifetimeDays).To(PointTo(Equal(30)))
			Expect(obj.Controllers.Project.StaleGracePeriodDays).To(PointTo(Equal(14)))
			Expect(obj.Controllers.Project.StaleExpirationTimeDays).To(PointTo(Equal(90)))
			Expect(obj.Controllers.Project.StaleSyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: 12 * time.Hour})))
			Expect(obj.Controllers.Project.Quotas).To(BeNil())

			Expect(obj.Controllers.Quota).NotTo(BeNil())
			Expect(obj.Controllers.Quota.ConcurrentSyncs).NotTo(BeNil())
			Expect(obj.Controllers.Quota.ConcurrentSyncs).To(PointTo(Equal(5)))

			Expect(obj.Controllers.SecretBinding).NotTo(BeNil())
			Expect(obj.Controllers.SecretBinding.ConcurrentSyncs).NotTo(BeNil())
			Expect(obj.Controllers.SecretBinding.ConcurrentSyncs).To(PointTo(Equal(5)))

			Expect(obj.Controllers.Seed).NotTo(BeNil())
			Expect(obj.Controllers.SeedExtensionsCheck).NotTo(BeNil())
			Expect(obj.Controllers.SeedBackupBucketsCheck).NotTo(BeNil())

			Expect(obj.Controllers.ShootMaintenance.ConcurrentSyncs).NotTo(BeNil())
			Expect(obj.Controllers.ShootMaintenance.ConcurrentSyncs).To(PointTo(Equal(5)))

			Expect(obj.Controllers.ShootQuota.ConcurrentSyncs).NotTo(BeNil())
			Expect(obj.Controllers.ShootQuota.ConcurrentSyncs).To(PointTo(Equal(5)))

			Expect(obj.Controllers.ShootReference).NotTo(BeNil())
			Expect(obj.Controllers.ShootReference.ConcurrentSyncs).NotTo(BeNil())
			Expect(obj.Controllers.ShootReference.ConcurrentSyncs).To(PointTo(Equal(5)))

			Expect(obj.Controllers.ShootRetry).NotTo(BeNil())
			Expect(obj.Controllers.ShootRetry.ConcurrentSyncs).NotTo(BeNil())
			Expect(obj.Controllers.ShootRetry.ConcurrentSyncs).To(PointTo(Equal(5)))

			Expect(obj.Controllers.ShootConditions).NotTo(BeNil())
			Expect(obj.Controllers.ShootConditions.ConcurrentSyncs).NotTo(BeNil())
			Expect(obj.Controllers.ShootConditions.ConcurrentSyncs).To(PointTo(Equal(5)))

			Expect(obj.Controllers.ShootStatusLabel).NotTo(BeNil())
			Expect(obj.Controllers.ShootStatusLabel.ConcurrentSyncs).NotTo(BeNil())
			Expect(obj.Controllers.ShootStatusLabel.ConcurrentSyncs).To(PointTo(Equal(5)))

			Expect(obj.LogLevel).To(Equal(logger.InfoLevel))
			Expect(obj.LogFormat).To(Equal(logger.FormatJSON))

			Expect(obj.Server.HealthProbes.BindAddress).To(BeEmpty())
			Expect(obj.Server.HealthProbes.Port).To(Equal(2718))
			Expect(obj.Server.Metrics.BindAddress).To(BeEmpty())
			Expect(obj.Server.Metrics.Port).To(Equal(2719))
		})

		It("should correctly default the project quota configuration", func() {
			fooSelector, _ := metav1.ParseToLabelSelector("role = foo")

			obj.Controllers = ControllerManagerControllerConfiguration{
				Project: &ProjectControllerConfiguration{
					Quotas: []QuotaConfiguration{
						{
							ProjectSelector: fooSelector,
						},
						{},
					},
				},
			}
			SetObjectDefaults_ControllerManagerConfiguration(obj)

			Expect(obj.Controllers.Project.Quotas[0].ProjectSelector).To(Equal(fooSelector))
			Expect(obj.Controllers.Project.Quotas[1].ProjectSelector).To(Equal(&metav1.LabelSelector{}))
		})

		Describe("GardenClientConnection", func() {
			It("should not default ContentType and AcceptContentTypes", func() {
				SetObjectDefaults_ControllerManagerConfiguration(obj)

				// ContentType fields will be defaulted by client constructors / controller-runtime based on whether a
				// given APIGroup supports protobuf or not. defaults must not touch these, otherwise the integelligent
				// logic will be overwritten
				Expect(obj.GardenClientConnection.ContentType).To(BeEmpty())
				Expect(obj.GardenClientConnection.AcceptContentTypes).To(BeEmpty())
			})
			It("should correctly default GardenClientConnection", func() {
				SetObjectDefaults_ControllerManagerConfiguration(obj)
				Expect(obj.GardenClientConnection).To(Equal(componentbaseconfigv1alpha1.ClientConnectionConfiguration{
					QPS:   50.0,
					Burst: 100,
				}))
			})
		})

		Describe("leader election settings", func() {
			It("should correctly default leader election settings", func() {
				SetObjectDefaults_ControllerManagerConfiguration(obj)

				Expect(obj.LeaderElection).NotTo(BeNil())
				Expect(obj.LeaderElection.LeaderElect).To(PointTo(BeTrue()))
				Expect(obj.LeaderElection.LeaseDuration).To(Equal(metav1.Duration{Duration: 15 * time.Second}))
				Expect(obj.LeaderElection.RenewDeadline).To(Equal(metav1.Duration{Duration: 10 * time.Second}))
				Expect(obj.LeaderElection.RetryPeriod).To(Equal(metav1.Duration{Duration: 2 * time.Second}))
				Expect(obj.LeaderElection.ResourceLock).To(Equal("leases"))
				Expect(obj.LeaderElection.ResourceNamespace).To(Equal("garden"))
				Expect(obj.LeaderElection.ResourceName).To(Equal("gardener-controller-manager-leader-election"))
			})
			It("should not overwrite custom settings", func() {
				expectedLeaderElection := &componentbaseconfigv1alpha1.LeaderElectionConfiguration{
					LeaderElect:       pointer.Bool(true),
					ResourceLock:      "foo",
					RetryPeriod:       metav1.Duration{Duration: 40 * time.Second},
					RenewDeadline:     metav1.Duration{Duration: 41 * time.Second},
					LeaseDuration:     metav1.Duration{Duration: 42 * time.Second},
					ResourceNamespace: "other-garden-ns",
					ResourceName:      "lock-object",
				}
				obj.LeaderElection = expectedLeaderElection.DeepCopy()
				SetObjectDefaults_ControllerManagerConfiguration(obj)

				Expect(obj.LeaderElection).To(Equal(expectedLeaderElection))
			})
		})
	})

	Describe("#SetDefaults_EventControllerConfiguration", func() {
		It("should correctly default the Event Controller configuration", func() {
			obj := &EventControllerConfiguration{}

			SetDefaults_EventControllerConfiguration(obj)
			Expect(obj.ConcurrentSyncs).NotTo(BeNil())
			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(5)))
			Expect(obj.TTLNonShootEvents).To(PointTo(Equal(metav1.Duration{Duration: time.Hour})))
		})
	})

	Describe("#SetDefaults_ShootRetryControllerConfiguration", func() {
		It("should correctly default the ShootRetry Controller configuration", func() {
			obj := &ShootRetryControllerConfiguration{}

			SetDefaults_ShootRetryControllerConfiguration(obj)
			Expect(obj.RetryPeriod).To(PointTo(Equal(metav1.Duration{Duration: 10 * time.Minute})))
			Expect(obj.RetryJitterPeriod).To(PointTo(Equal(metav1.Duration{Duration: 5 * time.Minute})))
		})
	})

	Describe("#SetDefaults_ManagedSeedSetControllerConfiguration", func() {
		It("should correctly default the ManagedSeedSet Controller configuration", func() {
			obj := &ManagedSeedSetControllerConfiguration{}

			SetDefaults_ManagedSeedSetControllerConfiguration(obj)
			Expect(obj.MaxShootRetries).To(PointTo(Equal(3)))
		})
	})

	Describe("#SetDefaults_ShootHibernationControllerConfiguration", func() {
		It("should correctly default the ShootHibernation Controller configuration", func() {
			obj := &ShootHibernationControllerConfiguration{}

			SetDefaults_ShootHibernationControllerConfiguration(obj)
			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(5)))
			Expect(obj.TriggerDeadlineDuration).To(PointTo(Equal(metav1.Duration{Duration: 2 * time.Hour})))
		})
	})

	Describe("#SetDefaults_SeedExtensionsCheckControllerConfiguration", func() {
		It("should correctly default the SeedExtensionsCheck Controller configuration", func() {
			obj := &SeedExtensionsCheckControllerConfiguration{}

			SetDefaults_SeedExtensionsCheckControllerConfiguration(obj)
			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(5)))
			Expect(obj.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: 30 * time.Second})))
		})
	})

	Describe("#SetDefaults_SeedBackupBucketsCheckControllerConfiguration", func() {
		It("should correctly default the SeedBackupBucketsCheck Controller configuration", func() {
			obj := &SeedBackupBucketsCheckControllerConfiguration{}

			SetDefaults_SeedBackupBucketsCheckControllerConfiguration(obj)
			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(5)))
			Expect(obj.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: 30 * time.Second})))
		})
	})

	Describe("#SetDefaults_CertificateSigningRequestControllerConfiguration", func() {
		It("should correctly default the CertificateSigningRequest Controller configuration", func() {
			obj := &CertificateSigningRequestControllerConfiguration{}

			SetDefaults_CertificateSigningRequestControllerConfiguration(obj)
			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(5)))
		})
	})

	Describe("#SetDefaults_SeedControllerConfiguration", func() {
		It("should correctly default the Seed Controller configuration", func() {
			obj := &SeedControllerConfiguration{}

			SetDefaults_SeedControllerConfiguration(obj)
			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(5)))
			Expect(obj.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: 10 * time.Second})))
			Expect(obj.MonitorPeriod).To(PointTo(Equal(metav1.Duration{Duration: 40 * time.Second})))
			Expect(obj.ShootMonitorPeriod).To(PointTo(Equal(metav1.Duration{Duration: 200 * time.Second})))
		})
	})
})

var _ = Describe("Constants", func() {
	It("should have the same values as the corresponding constants in the logger package", func() {
		Expect(LogLevelDebug).To(Equal(logger.DebugLevel))
		Expect(LogLevelInfo).To(Equal(logger.InfoLevel))
		Expect(LogLevelError).To(Equal(logger.ErrorLevel))
		Expect(LogFormatJSON).To(Equal(logger.FormatJSON))
		Expect(LogFormatText).To(Equal(logger.FormatText))
	})
})
