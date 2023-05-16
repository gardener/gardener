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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
)

var _ = Describe("Defaults", func() {
	Describe("GardenletConfiguration", func() {
		var obj *GardenletConfiguration

		BeforeEach(func() {
			obj = &GardenletConfiguration{}
		})

		It("should default the gardenlet configuration", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.GardenClientConnection).NotTo(BeNil())
			Expect(obj.SeedClientConnection).NotTo(BeNil())
			Expect(obj.ShootClientConnection).NotTo(BeNil())
			Expect(obj.Controllers.BackupBucket).NotTo(BeNil())
			Expect(obj.Controllers.BackupEntry).NotTo(BeNil())
			Expect(obj.Controllers.Bastion).NotTo(BeNil())
			Expect(obj.Controllers.ControllerInstallation).NotTo(BeNil())
			Expect(obj.Controllers.ControllerInstallationCare).NotTo(BeNil())
			Expect(obj.Controllers.ControllerInstallationRequired).NotTo(BeNil())
			Expect(obj.Controllers.Seed).NotTo(BeNil())
			Expect(obj.Controllers.Shoot).NotTo(BeNil())
			Expect(obj.Controllers.ShootCare).NotTo(BeNil())
			Expect(obj.Controllers.SeedCare).NotTo(BeNil())
			Expect(obj.Controllers.ShootStateSync).NotTo(BeNil())
			Expect(obj.Controllers.ManagedSeed).NotTo(BeNil())
			Expect(obj.LeaderElection).NotTo(BeNil())
			Expect(obj.LogLevel).To(Equal(logger.InfoLevel))
			Expect(obj.LogFormat).To(Equal(logger.FormatJSON))
			Expect(obj.Server.HealthProbes.BindAddress).To(BeEmpty())
			Expect(obj.Server.HealthProbes.Port).To(Equal(2728))
			Expect(obj.Server.Metrics.BindAddress).To(BeEmpty())
			Expect(obj.Server.Metrics.Port).To(Equal(2729))
			Expect(obj.SNI).ToNot(BeNil())
			Expect(obj.Monitoring).ToNot(BeNil())
			Expect(obj.SNI.Ingress).ToNot(BeNil())
			Expect(obj.SNI.Ingress.Namespace).To(PointTo(Equal("istio-ingress")))
			Expect(obj.SNI.Ingress.ServiceName).To(PointTo(Equal("istio-ingressgateway")))
			Expect(obj.SNI.Ingress.Labels).To(Equal(map[string]string{
				"app":   "istio-ingressgateway",
				"istio": "ingressgateway",
			}))
		})

		It("should default the gardenlets exposure class handlers sni config", func() {
			obj.ExposureClassHandlers = []ExposureClassHandler{
				{Name: "test1"},
				{Name: "test2", SNI: &SNI{}},
			}

			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.ExposureClassHandlers[0].SNI).ToNot(BeNil())
			Expect(obj.ExposureClassHandlers[0].SNI.Ingress).ToNot(BeNil())
			Expect(obj.ExposureClassHandlers[0].SNI.Ingress.Namespace).ToNot(BeNil())
			Expect(*obj.ExposureClassHandlers[0].SNI.Ingress.Namespace).To(Equal(fmt.Sprintf("istio-ingress-handler-%s", obj.ExposureClassHandlers[0].Name)))
			Expect(*obj.ExposureClassHandlers[0].SNI.Ingress.ServiceName).To(Equal("istio-ingressgateway"))
			Expect(obj.ExposureClassHandlers[0].SNI.Ingress.Labels).To(Equal(map[string]string{
				"app":                 "istio-ingressgateway",
				"gardener.cloud/role": "exposureclass-handler",
			}))

			Expect(obj.ExposureClassHandlers[1].SNI.Ingress).ToNot(BeNil())
			Expect(obj.ExposureClassHandlers[1].SNI.Ingress.Namespace).ToNot(BeNil())
			Expect(*obj.ExposureClassHandlers[1].SNI.Ingress.Namespace).To(Equal(fmt.Sprintf("istio-ingress-handler-%s", obj.ExposureClassHandlers[1].Name)))
			Expect(*obj.ExposureClassHandlers[1].SNI.Ingress.ServiceName).To(Equal("istio-ingressgateway"))
			Expect(obj.ExposureClassHandlers[1].SNI.Ingress.Labels).To(Equal(map[string]string{
				"app":                 "istio-ingressgateway",
				"gardener.cloud/role": "exposureclass-handler",
			}))
		})

		Describe("ClientConnection settings", func() {
			It("should not default ContentType and AcceptContentTypes", func() {
				SetObjectDefaults_GardenletConfiguration(obj)

				// ContentType fields will be defaulted by client constructors / controller-runtime based on whether a
				// given APIGroup supports protobuf or not. defaults must not touch these, otherwise the integelligent
				// logic will be overwritten
				Expect(obj.GardenClientConnection.ContentType).To(BeEmpty())
				Expect(obj.GardenClientConnection.AcceptContentTypes).To(BeEmpty())
				Expect(obj.SeedClientConnection.ContentType).To(BeEmpty())
				Expect(obj.SeedClientConnection.AcceptContentTypes).To(BeEmpty())
				Expect(obj.ShootClientConnection.ContentType).To(BeEmpty())
				Expect(obj.ShootClientConnection.AcceptContentTypes).To(BeEmpty())
			})

			It("should correctly default GardenClientConnection", func() {
				SetObjectDefaults_GardenletConfiguration(obj)
				Expect(obj.GardenClientConnection).To(Equal(&GardenClientConnection{
					ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
						QPS:   50.0,
						Burst: 100,
					},
					KubeconfigValidity: &KubeconfigValidity{
						AutoRotationJitterPercentageMin: pointer.Int32(70),
						AutoRotationJitterPercentageMax: pointer.Int32(90),
					},
				}))
				Expect(obj.SeedClientConnection).To(Equal(&SeedClientConnection{
					ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
						QPS:   50.0,
						Burst: 100,
					},
				}))
				Expect(obj.ShootClientConnection).To(Equal(&ShootClientConnection{
					ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
						QPS:   50.0,
						Burst: 100,
					},
				}))
			})
		})

		Describe("leader election settings", func() {
			It("should correctly default leader election settings", func() {
				SetObjectDefaults_GardenletConfiguration(obj)

				Expect(obj.LeaderElection).NotTo(BeNil())
				Expect(obj.LeaderElection.LeaderElect).To(PointTo(BeTrue()))
				Expect(obj.LeaderElection.LeaseDuration).To(Equal(metav1.Duration{Duration: 15 * time.Second}))
				Expect(obj.LeaderElection.RenewDeadline).To(Equal(metav1.Duration{Duration: 10 * time.Second}))
				Expect(obj.LeaderElection.RetryPeriod).To(Equal(metav1.Duration{Duration: 2 * time.Second}))
				Expect(obj.LeaderElection.ResourceLock).To(Equal("leases"))
				Expect(obj.LeaderElection.ResourceNamespace).To(Equal("garden"))
				Expect(obj.LeaderElection.ResourceName).To(Equal("gardenlet-leader-election"))
			})
			It("should not overwrite custom settings", func() {
				expectedLeaderElection := &componentbaseconfigv1alpha1.LeaderElectionConfiguration{
					LeaderElect:       pointer.Bool(true),
					ResourceLock:      "foo",
					RetryPeriod:       metav1.Duration{Duration: 40 * time.Second},
					RenewDeadline:     metav1.Duration{Duration: 41 * time.Second},
					LeaseDuration:     metav1.Duration{Duration: 42 * time.Second},
					ResourceName:      "lock-object",
					ResourceNamespace: "other-garden-ns",
				}
				obj.LeaderElection = expectedLeaderElection.DeepCopy()
				SetObjectDefaults_GardenletConfiguration(obj)

				Expect(obj.LeaderElection).To(Equal(expectedLeaderElection))
			})
		})

		Describe("Logging settings", func() {
			It("should correctly default Logging configuration", func() {
				SetObjectDefaults_GardenletConfiguration(obj)
				Expect(obj.Logging).NotTo(BeNil())
				Expect(obj.Logging.Enabled).To(PointTo(Equal(false)))
				Expect(obj.Logging.Vali).NotTo(BeNil())
				Expect(obj.Logging.Vali.Enabled).To(PointTo(Equal(false)))
				Expect(obj.Logging.Vali.Garden).NotTo(BeNil())
				Expect(obj.Logging.Vali.Garden.Storage).To(PointTo(Equal(resource.MustParse("100Gi"))))
				Expect(obj.Logging.ShootEventLogging).NotTo(BeNil())
				Expect(obj.Logging.ShootEventLogging.Enabled).To(PointTo(Equal(false)))
			})

			It("should not overwrite custom settings", func() {
				gardenValiStorage := resource.MustParse("10Gi")
				expectedLogging := &Logging{
					Enabled: pointer.Bool(true),
					Vali: &Vali{
						Enabled: pointer.Bool(false),
						Garden: &GardenVali{
							Storage: &gardenValiStorage,
						},
					},
					ShootNodeLogging: &ShootNodeLogging{
						ShootPurposes: []gardencorev1beta1.ShootPurpose{
							"development",
							"evaluation",
						},
					},
					ShootEventLogging: &ShootEventLogging{
						Enabled: pointer.Bool(false),
					},
				}

				obj.Logging = expectedLogging.DeepCopy()
				SetObjectDefaults_GardenletConfiguration(obj)

				Expect(obj.Logging).To(Equal(expectedLogging))
			})
		})

		Describe("#SetDefaults_ETCDConfig", func() {
			It("should correctly default ETCDConfig configuration", func() {
				SetObjectDefaults_GardenletConfiguration(obj)
				Expect(obj.ETCDConfig).NotTo(BeNil())
				Expect(obj.ETCDConfig.ETCDController).NotTo(BeNil())
				Expect(obj.ETCDConfig.ETCDController.Workers).To(PointTo(Equal(int64(50))))
				Expect(obj.ETCDConfig.CustodianController).NotTo(BeNil())
				Expect(obj.ETCDConfig.CustodianController.Workers).To(PointTo(Equal(int64(10))))
				Expect(obj.ETCDConfig.BackupCompactionController).NotTo(BeNil())
				Expect(obj.ETCDConfig.BackupCompactionController.Workers).To(PointTo(Equal(int64(3))))
				Expect(obj.ETCDConfig.BackupCompactionController.EnableBackupCompaction).To(PointTo(Equal(false)))
				Expect(obj.ETCDConfig.BackupCompactionController.EventsThreshold).To(PointTo(Equal(int64(1000000))))
			})
		})
	})

	Describe("#SetDefaults_GardenClientConnection", func() {
		var obj *GardenClientConnection

		BeforeEach(func() {
			obj = &GardenClientConnection{}
		})

		It("should default the configuration", func() {
			SetDefaults_GardenClientConnection(obj)

			Expect(obj.KubeconfigValidity).NotTo(BeNil())
		})
	})

	Describe("#SetDefaults_KubeconfigValidity", func() {
		var obj *KubeconfigValidity

		BeforeEach(func() {
			obj = &KubeconfigValidity{}
		})

		It("should default the configuration", func() {
			SetDefaults_KubeconfigValidity(obj)

			Expect(obj.Validity).To(BeNil())
			Expect(obj.AutoRotationJitterPercentageMin).To(PointTo(Equal(int32(70))))
			Expect(obj.AutoRotationJitterPercentageMax).To(PointTo(Equal(int32(90))))
		})
	})

	Describe("#SetDefaults_ManagedSeedControllerConfiguration", func() {
		var obj *ManagedSeedControllerConfiguration

		BeforeEach(func() {
			obj = &ManagedSeedControllerConfiguration{}
		})

		It("should default the configuration", func() {
			SetDefaults_ManagedSeedControllerConfiguration(obj)

			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(DefaultControllerConcurrentSyncs)))
			Expect(obj.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: 1 * time.Hour})))
			Expect(obj.WaitSyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: 15 * time.Second})))
			Expect(obj.SyncJitterPeriod).To(PointTo(Equal(metav1.Duration{Duration: 5 * time.Minute})))
			Expect(obj.JitterUpdates).To(PointTo(BeFalse()))
		})
	})

	Describe("#SetDefaults_SeedControllerConfiguration", func() {
		var obj *SeedControllerConfiguration

		BeforeEach(func() {
			obj = &SeedControllerConfiguration{}
		})

		It("should default the configuration", func() {
			SetDefaults_SeedControllerConfiguration(obj)

			Expect(obj.SyncPeriod).To(PointTo(Equal(DefaultControllerSyncPeriod)))
			Expect(obj.LeaseResyncSeconds).To(PointTo(Equal(int32(2))))
			Expect(obj.LeaseResyncMissThreshold).To(PointTo(Equal(int32(10))))
		})
	})

	Describe("#SetDefaults_SeedCareControllerConfiguration", func() {
		var obj *SeedCareControllerConfiguration

		BeforeEach(func() {
			obj = &SeedCareControllerConfiguration{}
		})

		It("should default the configuration", func() {
			SetDefaults_SeedCareControllerConfiguration(obj)

			Expect(obj.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: 30 * time.Second})))
		})
	})

	Describe("#SetDefaults_ShootControllerConfiguration", func() {
		var obj *ShootControllerConfiguration

		BeforeEach(func() {
			obj = &ShootControllerConfiguration{}
		})

		It("should default the configuration", func() {
			SetDefaults_ShootControllerConfiguration(obj)

			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(20)))
			Expect(obj.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Hour})))
			Expect(obj.RespectSyncPeriodOverwrite).To(PointTo(Equal(false)))
			Expect(obj.ReconcileInMaintenanceOnly).To(PointTo(Equal(false)))
			Expect(obj.RetryDuration).To(PointTo(Equal(metav1.Duration{Duration: 12 * time.Hour})))
			Expect(obj.DNSEntryTTLSeconds).To(PointTo(Equal(int64(120))))
		})
	})

	Describe("#SetDefaults_ShootCareControllerConfiguration", func() {
		var obj *ShootCareControllerConfiguration

		BeforeEach(func() {
			obj = &ShootCareControllerConfiguration{}
		})

		It("should default the configuration", func() {
			SetDefaults_ShootCareControllerConfiguration(obj)

			Expect(obj.SyncPeriod).To(PointTo(Equal(DefaultControllerSyncPeriod)))
			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(DefaultControllerConcurrentSyncs)))
			Expect(obj.StaleExtensionHealthChecks).To(PointTo(Equal(StaleExtensionHealthChecks{Enabled: true})))
		})
	})

	Describe("#SetDefaults_ShootSecretControllerConfiguration", func() {
		var obj *ShootSecretControllerConfiguration

		BeforeEach(func() {
			obj = &ShootSecretControllerConfiguration{}
		})

		It("should default the configuration", func() {
			SetDefaults_ShootSecretControllerConfiguration(obj)

			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(5)))
		})
	})

	Describe("#SetDefaults_BackupEntryControllerConfiguration", func() {
		var obj *BackupEntryControllerConfiguration

		BeforeEach(func() {
			obj = &BackupEntryControllerConfiguration{}
		})

		It("should default the configuration", func() {
			SetDefaults_BackupEntryControllerConfiguration(obj)

			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(20)))
			Expect(obj.DeletionGracePeriodHours).To(PointTo(Equal(0)))
			Expect(obj.DeletionGracePeriodShootPurposes).To(BeEmpty())
		})
	})

	Describe("#SetDefaults_BastionControllerConfiguration", func() {
		var obj *BastionControllerConfiguration

		BeforeEach(func() {
			obj = &BastionControllerConfiguration{}
		})

		It("should default the configuration", func() {
			SetDefaults_BastionControllerConfiguration(obj)

			Expect(obj.ConcurrentSyncs).To(PointTo(Equal(20)))
		})
	})

	Describe("#SetDefaults_MonitoringConfig", func() {
		var obj *MonitoringConfig

		BeforeEach(func() {
			obj = &MonitoringConfig{}
		})

		It("should default to the configuration", func() {
			SetDefaults_MonitoringConfig(obj)

			Expect(obj.Shoot).ToNot(BeNil())
		})
	})

	Describe("#SetDefaults_ShootMonitoringConfig", func() {
		var obj *ShootMonitoringConfig

		BeforeEach(func() {
			obj = &ShootMonitoringConfig{}
		})

		It("should default to the configuration", func() {
			SetDefaults_ShootMonitoringConfig(obj)

			Expect(obj).ToNot(BeNil())
			Expect(*obj.Enabled).To(BeTrue())
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
