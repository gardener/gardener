// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/operator/apis/config/v1alpha1"
)

var _ = Describe("Defaults", func() {
	var obj *OperatorConfiguration

	BeforeEach(func() {
		obj = &OperatorConfiguration{}
	})

	Describe("OperatorConfiguration defaulting", func() {
		It("should correctly default the configuration", func() {
			SetObjectDefaults_OperatorConfiguration(obj)

			Expect(obj.LogLevel).To(Equal(logger.InfoLevel))
			Expect(obj.LogFormat).To(Equal(logger.FormatJSON))
		})

		It("should not overwrite already set values for OperatorConfiguration", func() {
			var (
				expectedLogLevel  = "foo"
				expectedLogFormat = "bar"
			)

			obj.LogLevel = expectedLogLevel
			obj.LogFormat = expectedLogFormat

			SetObjectDefaults_OperatorConfiguration(obj)

			Expect(obj.LogLevel).To(Equal(expectedLogLevel))
			Expect(obj.LogFormat).To(Equal(expectedLogFormat))
		})
	})

	Describe("ServerConfiguration defaulting", func() {
		It("should correctly default the Server configuration", func() {
			SetObjectDefaults_OperatorConfiguration(obj)

			Expect(obj.Server.Webhooks.BindAddress).To(BeEmpty())
			Expect(obj.Server.Webhooks.Port).To(Equal(2750))
			Expect(obj.Server.HealthProbes.BindAddress).To(BeEmpty())
			Expect(obj.Server.HealthProbes.Port).To(Equal(2751))
			Expect(obj.Server.Metrics.BindAddress).To(BeEmpty())
			Expect(obj.Server.Metrics.Port).To(Equal(2752))
		})

		It("should not overwrite already set values for Server configuration", func() {
			expectedServer := ServerConfiguration{
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
			obj.Server = expectedServer

			SetObjectDefaults_OperatorConfiguration(obj)

			Expect(obj.Server).To(Equal(expectedServer))
		})
	})

	Describe("RuntimeClientConnection defaulting", func() {
		It("should not default ContentType and AcceptContentTypes", func() {
			SetObjectDefaults_OperatorConfiguration(obj)

			// ContentType fields will be defaulted by client constructors / controller-runtime based on whether a
			// given APIGroup supports protobuf or not. defaults must not touch these, otherwise the intelligent
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

		It("should not overwrite already set values for RuntimeClientConnection", func() {
			obj.RuntimeClientConnection = componentbaseconfigv1alpha1.ClientConnectionConfiguration{
				QPS:   60.0,
				Burst: 90,
			}

			SetObjectDefaults_OperatorConfiguration(obj)

			Expect(obj.RuntimeClientConnection).To(Equal(componentbaseconfigv1alpha1.ClientConnectionConfiguration{
				QPS:   60.0,
				Burst: 90,
			}))
		})
	})

	Describe("VirtualClientConnection defaulting", func() {
		It("should not default ContentType and AcceptContentTypes", func() {
			SetObjectDefaults_OperatorConfiguration(obj)

			// ContentType fields will be defaulted by client constructors / controller-runtime based on whether a
			// given APIGroup supports protobuf or not. defaults must not touch these, otherwise the intelligent
			// logic will be overwritten
			Expect(obj.VirtualClientConnection.ContentType).To(BeEmpty())
			Expect(obj.VirtualClientConnection.AcceptContentTypes).To(BeEmpty())
		})

		It("should correctly default VirtualClientConnection", func() {
			SetObjectDefaults_OperatorConfiguration(obj)

			Expect(obj.VirtualClientConnection).To(Equal(componentbaseconfigv1alpha1.ClientConnectionConfiguration{
				QPS:   100.0,
				Burst: 130,
			}))
		})

		It("should not overwrite already set values for VirtualClientConnection", func() {
			obj.VirtualClientConnection = componentbaseconfigv1alpha1.ClientConnectionConfiguration{
				QPS:   60.0,
				Burst: 90,
			}

			SetObjectDefaults_OperatorConfiguration(obj)

			Expect(obj.VirtualClientConnection).To(Equal(componentbaseconfigv1alpha1.ClientConnectionConfiguration{
				QPS:   60.0,
				Burst: 90,
			}))
		})
	})

	Describe("LeaderElection defaulting", func() {
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

		It("should not overwrite already set values for leader election settings", func() {
			expectedLeaderElection := componentbaseconfigv1alpha1.LeaderElectionConfiguration{
				LeaderElect:       ptr.To(true),
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

	Describe("Controller configuration defaulting", func() {
		Describe("Garden controller defaulting", func() {
			It("should default the Garden controller config", func() {
				SetObjectDefaults_OperatorConfiguration(obj)

				Expect(obj.Controllers.Garden.ConcurrentSyncs).To(PointTo(Equal(1)))
				Expect(obj.Controllers.Garden.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Hour})))
				Expect(obj.Controllers.Garden.ETCDConfig).NotTo(BeNil())
				Expect(obj.Controllers.Garden.ETCDConfig.ETCDController).NotTo(BeNil())
				Expect(obj.Controllers.Garden.ETCDConfig.ETCDController.Workers).To(PointTo(Equal(int64(50))))
				Expect(obj.Controllers.Garden.ETCDConfig.CustodianController).NotTo(BeNil())
				Expect(obj.Controllers.Garden.ETCDConfig.CustodianController.Workers).To(PointTo(Equal(int64(10))))
				Expect(obj.Controllers.Garden.ETCDConfig.BackupCompactionController).NotTo(BeNil())
				Expect(obj.Controllers.Garden.ETCDConfig.BackupCompactionController.Workers).To(PointTo(Equal(int64(3))))
				Expect(obj.Controllers.Garden.ETCDConfig.BackupCompactionController.EnableBackupCompaction).To(PointTo(Equal(false)))
				Expect(obj.Controllers.Garden.ETCDConfig.BackupCompactionController.EventsThreshold).To(PointTo(Equal(int64(1000000))))
				Expect(obj.Controllers.Garden.ETCDConfig.BackupCompactionController.MetricsScrapeWaitDuration).To(PointTo(Equal(metav1.Duration{Duration: 60 * time.Second})))
			})

			It("should not overwrite already set values for Garden controller config", func() {
				obj = &OperatorConfiguration{
					Controllers: ControllerConfiguration{
						Garden: GardenControllerConfig{
							ConcurrentSyncs: ptr.To(5),
							SyncPeriod:      &metav1.Duration{Duration: time.Second},
							ETCDConfig: &v1alpha1.ETCDConfig{
								ETCDController:      &v1alpha1.ETCDController{Workers: ptr.To[int64](5)},
								CustodianController: &v1alpha1.CustodianController{Workers: ptr.To[int64](5)},
								BackupCompactionController: &v1alpha1.BackupCompactionController{
									Workers:                   ptr.To[int64](4),
									EnableBackupCompaction:    ptr.To(true),
									EventsThreshold:           ptr.To[int64](900000),
									MetricsScrapeWaitDuration: &metav1.Duration{Duration: 30 * time.Second},
								},
							},
						},
					},
				}

				SetObjectDefaults_OperatorConfiguration(obj)

				Expect(obj.Controllers.Garden.ConcurrentSyncs).To(PointTo(Equal(5)))
				Expect(obj.Controllers.Garden.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Second})))
				Expect(obj.Controllers.Garden.ETCDConfig.ETCDController.Workers).To(PointTo(Equal(int64(5))))
				Expect(obj.Controllers.Garden.ETCDConfig.CustodianController.Workers).To(PointTo(Equal(int64(5))))
				Expect(obj.Controllers.Garden.ETCDConfig.BackupCompactionController.Workers).To(PointTo(Equal(int64(4))))
				Expect(obj.Controllers.Garden.ETCDConfig.BackupCompactionController.EnableBackupCompaction).To(PointTo(Equal(true)))
				Expect(obj.Controllers.Garden.ETCDConfig.BackupCompactionController.EventsThreshold).To(PointTo(Equal(int64(900000))))
				Expect(obj.Controllers.Garden.ETCDConfig.BackupCompactionController.MetricsScrapeWaitDuration).To(PointTo(Equal(metav1.Duration{Duration: 30 * time.Second})))
			})
		})

		Describe("GardenCare controller defaulting", func() {
			It("should default the GardenCare controller config", func() {
				SetObjectDefaults_OperatorConfiguration(obj)

				Expect(obj.Controllers.GardenCare.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Minute})))
			})

			It("should not overwrite already set values for GardenCare controller config", func() {
				obj = &OperatorConfiguration{
					Controllers: ControllerConfiguration{
						GardenCare: GardenCareControllerConfiguration{
							SyncPeriod: &metav1.Duration{Duration: time.Second},
						},
					},
				}

				SetObjectDefaults_OperatorConfiguration(obj)

				Expect(obj.Controllers.GardenCare.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Second})))
			})
		})

		Describe("Extension controller defaulting", func() {
			It("should default the Extension controller config", func() {
				SetObjectDefaults_OperatorConfiguration(obj)

				Expect(obj.Controllers.Extension.ConcurrentSyncs).To(PointTo(Equal(5)))
			})

			It("should not overwrite already set values for GardenCare controller config", func() {
				obj = &OperatorConfiguration{
					Controllers: ControllerConfiguration{
						Extension: ExtensionControllerConfiguration{
							ConcurrentSyncs: ptr.To(2),
						},
					},
				}

				SetObjectDefaults_OperatorConfiguration(obj)

				Expect(obj.Controllers.Extension.ConcurrentSyncs).To(PointTo(Equal(2)))
			})
		})

		Describe("ExtensionCare controller defaulting", func() {
			It("should default the ExtensionCare controller config", func() {
				SetObjectDefaults_OperatorConfiguration(obj)

				Expect(obj.Controllers.GardenCare.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Minute})))
			})

			It("should not overwrite already set values for ExtensionCare controller config", func() {
				obj = &OperatorConfiguration{
					Controllers: ControllerConfiguration{
						ExtensionCare: ExtensionCareControllerConfiguration{
							SyncPeriod: &metav1.Duration{Duration: time.Second},
						},
					},
				}

				SetObjectDefaults_OperatorConfiguration(obj)

				Expect(obj.Controllers.ExtensionCare.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Second})))
			})
		})

		Describe("ExtensionRequiredRuntime controller defaulting", func() {
			It("should default the ExtensionRequiredRuntime controller config", func() {
				SetObjectDefaults_OperatorConfiguration(obj)

				Expect(obj.Controllers.ExtensionRequiredRuntime.ConcurrentSyncs).To(PointTo(Equal(5)))
			})

			It("should not overwrite already set values for GardenCare controller config", func() {
				obj = &OperatorConfiguration{
					Controllers: ControllerConfiguration{
						ExtensionRequiredRuntime: ExtensionRequiredRuntimeControllerConfiguration{
							ConcurrentSyncs: ptr.To(2),
						},
					},
				}

				SetObjectDefaults_OperatorConfiguration(obj)

				Expect(obj.Controllers.ExtensionRequiredRuntime.ConcurrentSyncs).To(PointTo(Equal(2)))
			})
		})

		Describe("ExtensionRequiredVirtual controller defaulting", func() {
			It("should default the ExtensionRequiredVirtual controller config", func() {
				SetObjectDefaults_OperatorConfiguration(obj)

				Expect(obj.Controllers.ExtensionRequiredVirtual.ConcurrentSyncs).To(PointTo(Equal(5)))
			})

			It("should not overwrite already set values for GardenCare controller config", func() {
				obj = &OperatorConfiguration{
					Controllers: ControllerConfiguration{
						ExtensionRequiredVirtual: ExtensionRequiredVirtualControllerConfiguration{
							ConcurrentSyncs: ptr.To(2),
						},
					},
				}

				SetObjectDefaults_OperatorConfiguration(obj)

				Expect(obj.Controllers.ExtensionRequiredVirtual.ConcurrentSyncs).To(PointTo(Equal(2)))
			})
		})
	})
})
