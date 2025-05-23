// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
)

var _ = Describe("Defaults", func() {
	var obj *GardenletConfiguration

	BeforeEach(func() {
		obj = &GardenletConfiguration{}
	})

	Describe("GardenletConfiguration defaulting", func() {
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
			Expect(obj.Controllers.Gardenlet).NotTo(BeNil())
			Expect(obj.Controllers.Seed).NotTo(BeNil())
			Expect(obj.Controllers.Shoot).NotTo(BeNil())
			Expect(obj.Controllers.ShootCare).NotTo(BeNil())
			Expect(obj.Controllers.SeedCare).NotTo(BeNil())
			Expect(obj.Controllers.ShootState).NotTo(BeNil())
			Expect(obj.Controllers.ManagedSeed).NotTo(BeNil())
			Expect(obj.LeaderElection).NotTo(BeNil())
			Expect(obj.LogLevel).To(Equal(logger.InfoLevel))
			Expect(obj.LogFormat).To(Equal(logger.FormatJSON))
			Expect(obj.SNI).NotTo(BeNil())
			Expect(obj.Monitoring).NotTo(BeNil())
			Expect(obj.ETCDConfig).NotTo(BeNil())
		})

		It("should not overwrite already set values for the gardenlet configuration", func() {
			obj.LogLevel = LogLevelDebug
			obj.LogFormat = LogFormatText
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.LogLevel).To(Equal(logger.DebugLevel))
			Expect(obj.LogFormat).To(Equal(logger.FormatText))
		})
	})

	Describe("GardenClientConnection defaulting", func() {
		It("should default the garden client connection", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			// ContentType fields will be defaulted by client constructors / controller-runtime based on whether a
			// given APIGroup supports protobuf or not. defaults must not touch these, otherwise the intelligent
			// logic will be overwritten
			Expect(obj.GardenClientConnection.ContentType).To(BeEmpty())
			Expect(obj.GardenClientConnection.AcceptContentTypes).To(BeEmpty())
			Expect(obj.GardenClientConnection.KubeconfigValidity).NotTo(BeNil())
			Expect(obj.GardenClientConnection.ClientConnectionConfiguration.QPS).To(Equal(float32(50.0)))
			Expect(obj.GardenClientConnection.ClientConnectionConfiguration.Burst).To(Equal(int32(100)))
		})

		It("should not overwrite already set values for the garden client connection", func() {
			obj.GardenClientConnection = &GardenClientConnection{
				ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
					QPS:   60.0,
					Burst: 90,
				},
			}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.GardenClientConnection.ClientConnectionConfiguration.QPS).To(Equal(float32(60.0)))
			Expect(obj.GardenClientConnection.ClientConnectionConfiguration.Burst).To(Equal(int32(90)))
		})
	})

	Describe("KubeconfigValidity defaulting", func() {
		It("should default the kubeconfig validity", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.GardenClientConnection.KubeconfigValidity).To(Equal(&KubeconfigValidity{
				AutoRotationJitterPercentageMin: ptr.To[int32](70),
				AutoRotationJitterPercentageMax: ptr.To[int32](90),
			}))
		})

		It("should not overwrite already set values for the kubeconfig validity", func() {
			v := metav1.Duration{Duration: 2 * time.Minute}
			obj.GardenClientConnection = &GardenClientConnection{
				KubeconfigValidity: &KubeconfigValidity{
					Validity:                        &v,
					AutoRotationJitterPercentageMin: ptr.To[int32](10),
					AutoRotationJitterPercentageMax: ptr.To[int32](50),
				},
			}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.GardenClientConnection.KubeconfigValidity.Validity).To(PointTo(Equal(v)))
			Expect(obj.GardenClientConnection.KubeconfigValidity.AutoRotationJitterPercentageMin).To(PointTo(Equal(int32(10))))
			Expect(obj.GardenClientConnection.KubeconfigValidity.AutoRotationJitterPercentageMax).To(PointTo(Equal(int32(50))))
		})
	})

	Describe("SeedClientConnection defaulting", func() {
		It("should default the seed client connection", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			// ContentType fields will be defaulted by client constructors / controller-runtime based on whether a
			// given APIGroup supports protobuf or not. defaults must not touch these, otherwise the intelligent
			// logic will be overwritten
			Expect(obj.SeedClientConnection.ContentType).To(BeEmpty())
			Expect(obj.SeedClientConnection.AcceptContentTypes).To(BeEmpty())
			Expect(obj.SeedClientConnection.ClientConnectionConfiguration.QPS).To(Equal(float32(50.0)))
			Expect(obj.SeedClientConnection.ClientConnectionConfiguration.Burst).To(Equal(int32(100)))
		})

		It("should not overwrite already set values for the seed client connection", func() {
			obj.SeedClientConnection = &SeedClientConnection{
				ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
					QPS:   60.0,
					Burst: 90,
				},
			}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.SeedClientConnection.ClientConnectionConfiguration.QPS).To(Equal(float32(60.0)))
			Expect(obj.SeedClientConnection.ClientConnectionConfiguration.Burst).To(Equal(int32(90)))
		})
	})

	Describe("ShootClientConnection defaulting", func() {
		It("should default the shoot client connection", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			// ContentType fields will be defaulted by client constructors / controller-runtime based on whether a
			// given APIGroup supports protobuf or not. defaults must not touch these, otherwise the intelligent
			// logic will be overwritten
			Expect(obj.ShootClientConnection.ContentType).To(BeEmpty())
			Expect(obj.ShootClientConnection.AcceptContentTypes).To(BeEmpty())
			Expect(obj.ShootClientConnection.ClientConnectionConfiguration.QPS).To(Equal(float32(50.0)))
			Expect(obj.ShootClientConnection.ClientConnectionConfiguration.Burst).To(Equal(int32(100)))
		})

		It("should not overwrite already set values for the shoot client connection", func() {
			obj.ShootClientConnection = &ShootClientConnection{
				ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
					QPS:   60.0,
					Burst: 90,
				},
			}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.ShootClientConnection.ClientConnectionConfiguration.QPS).To(Equal(float32(60.0)))
			Expect(obj.ShootClientConnection.ClientConnectionConfiguration.Burst).To(Equal(int32(90)))
		})
	})

	Describe("BackupBucketControllerConfiguration defaulting", func() {
		It("should default the backup bucket controller configuration", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.BackupBucket.ConcurrentSyncs).To(PointTo(Equal(20)))
		})

		It("should not overwrite already set values for the backup bucket controller configuration", func() {
			obj.Controllers = &GardenletControllerConfiguration{
				BackupBucket: &BackupBucketControllerConfiguration{ConcurrentSyncs: ptr.To(10)},
			}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.BackupBucket.ConcurrentSyncs).To(PointTo(Equal(10)))
		})
	})

	Describe("BackupEntryControllerConfiguration defaulting", func() {
		It("should default the backup entry controller configuration", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.BackupEntry.ConcurrentSyncs).To(PointTo(Equal(20)))
			Expect(obj.Controllers.BackupEntry.DeletionGracePeriodHours).To(PointTo(Equal(0)))
			Expect(obj.Controllers.BackupEntry.DeletionGracePeriodShootPurposes).To(BeEmpty())
		})

		It("should not overwrite already set values for the backup entry controller configuration", func() {
			deletionGracePeriodShootPurposes := []gardencorev1beta1.ShootPurpose{gardencorev1beta1.ShootPurposeEvaluation}
			obj.Controllers = &GardenletControllerConfiguration{
				BackupEntry: &BackupEntryControllerConfiguration{
					ConcurrentSyncs:                  ptr.To(10),
					DeletionGracePeriodHours:         ptr.To(1),
					DeletionGracePeriodShootPurposes: deletionGracePeriodShootPurposes,
				},
			}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.BackupEntry.ConcurrentSyncs).To(PointTo(Equal(10)))
			Expect(obj.Controllers.BackupEntry.DeletionGracePeriodHours).To(PointTo(Equal(1)))
			Expect(obj.Controllers.BackupEntry.DeletionGracePeriodShootPurposes).To(Equal(deletionGracePeriodShootPurposes))
		})
	})

	Describe("BastionControllerConfiguration defaulting", func() {
		It("should default the bastion controller configuration", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.Bastion.ConcurrentSyncs).To(PointTo(Equal(20)))
		})

		It("should not overwrite already set values for the bastion controller configuration", func() {
			obj.Controllers = &GardenletControllerConfiguration{
				Bastion: &BastionControllerConfiguration{ConcurrentSyncs: ptr.To(10)},
			}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.Bastion.ConcurrentSyncs).To(PointTo(Equal(10)))
		})
	})

	Describe("ControllerInstallationControllerConfiguration defaulting", func() {
		It("should default the controller installation controller configuration", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.ControllerInstallation.ConcurrentSyncs).To(PointTo(Equal(20)))
		})

		It("should not overwrite already set values for the controller installation controller configuration", func() {
			obj.Controllers = &GardenletControllerConfiguration{
				ControllerInstallation: &ControllerInstallationControllerConfiguration{ConcurrentSyncs: ptr.To(10)},
			}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.ControllerInstallation.ConcurrentSyncs).To(PointTo(Equal(10)))
		})
	})

	Describe("ControllerInstallationCareControllerConfiguration defaulting", func() {
		It("should default the controller installation care controller configuration", func() {
			v := metav1.Duration{Duration: 30 * time.Second}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.ControllerInstallationCare.ConcurrentSyncs).To(PointTo(Equal(20)))
			Expect(obj.Controllers.ControllerInstallationCare.SyncPeriod).To(PointTo(Equal(v)))
		})

		It("should not overwrite already set values for the controller installation care controller configuration", func() {
			v := metav1.Duration{Duration: 2 * time.Minute}
			obj.Controllers = &GardenletControllerConfiguration{
				ControllerInstallationCare: &ControllerInstallationCareControllerConfiguration{
					ConcurrentSyncs: ptr.To(10),
					SyncPeriod:      &v,
				},
			}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.ControllerInstallationCare.ConcurrentSyncs).To(PointTo(Equal(10)))
			Expect(obj.Controllers.ControllerInstallationCare.SyncPeriod).To(PointTo(Equal(v)))
		})
	})

	Describe("ControllerInstallationRequiredControllerConfiguration defaulting", func() {
		It("should default the controller installation required controller configuration", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.ControllerInstallationRequired.ConcurrentSyncs).To(PointTo(Equal(1)))
		})

		It("should not overwrite already set values for the controller installation required controller configuration", func() {
			obj.Controllers = &GardenletControllerConfiguration{
				ControllerInstallationRequired: &ControllerInstallationRequiredControllerConfiguration{ConcurrentSyncs: ptr.To(10)},
			}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.ControllerInstallationRequired.ConcurrentSyncs).To(PointTo(Equal(10)))
		})
	})

	Describe("GardenletObjectControllerConfiguration defaulting", func() {
		It("should default the managed seed controller configuration", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.Gardenlet.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: 1 * time.Hour})))
		})

		It("should not overwrite already set values for the managed seed controller configuration", func() {
			v := metav1.Duration{Duration: 2 * time.Minute}
			obj.Controllers = &GardenletControllerConfiguration{
				Gardenlet: &GardenletObjectControllerConfiguration{
					SyncPeriod: &v,
				},
			}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.Gardenlet.SyncPeriod).To(PointTo(Equal(v)))
		})
	})

	Describe("SeedControllerConfiguration defaulting", func() {
		It("should default the seed controller configuration", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.Seed.SyncPeriod).To(PointTo(Equal(DefaultControllerSyncPeriod)))
			Expect(obj.Controllers.Seed.LeaseResyncSeconds).To(PointTo(Equal(int32(2))))
			Expect(obj.Controllers.Seed.LeaseResyncMissThreshold).To(PointTo(Equal(int32(10))))
		})

		It("should not overwrite already set values for the seed controller configuration", func() {
			syncPeriod := metav1.Duration{Duration: 2 * time.Minute}
			obj.Controllers = &GardenletControllerConfiguration{
				Seed: &SeedControllerConfiguration{
					SyncPeriod:               &syncPeriod,
					LeaseResyncSeconds:       ptr.To[int32](1),
					LeaseResyncMissThreshold: ptr.To[int32](5),
				},
			}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.Seed.SyncPeriod).To(PointTo(Equal(syncPeriod)))
			Expect(obj.Controllers.Seed.LeaseResyncSeconds).To(PointTo(Equal(int32(1))))
			Expect(obj.Controllers.Seed.LeaseResyncMissThreshold).To(PointTo(Equal(int32(5))))
		})
	})

	Describe("SeedCareControllerConfiguration defaulting", func() {
		It("should default the seed care controller configuration", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.SeedCare.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: 30 * time.Second})))
		})

		It("should not overwrite already set values for the seed care controller configuration", func() {
			syncPeriod := metav1.Duration{Duration: 2 * time.Minute}
			obj.Controllers = &GardenletControllerConfiguration{
				SeedCare: &SeedCareControllerConfiguration{SyncPeriod: &syncPeriod},
			}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.SeedCare.SyncPeriod).To(PointTo(Equal(syncPeriod)))
		})
	})

	Describe("ShootControllerConfiguration defaulting", func() {
		It("should default the shoot controller configuration", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.Shoot.ConcurrentSyncs).To(PointTo(Equal(20)))
			Expect(obj.Controllers.Shoot.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Hour})))
			Expect(obj.Controllers.Shoot.RespectSyncPeriodOverwrite).To(PointTo(Equal(false)))
			Expect(obj.Controllers.Shoot.ReconcileInMaintenanceOnly).To(PointTo(Equal(false)))
			Expect(obj.Controllers.Shoot.RetryDuration).To(PointTo(Equal(metav1.Duration{Duration: 12 * time.Hour})))
			Expect(obj.Controllers.Shoot.DNSEntryTTLSeconds).To(PointTo(Equal(int64(120))))
		})

		It("should not overwrite already set values for the shoot controller configuration", func() {
			v := metav1.Duration{Duration: 2 * time.Hour}
			obj.Controllers = &GardenletControllerConfiguration{
				Shoot: &ShootControllerConfiguration{
					ConcurrentSyncs:            ptr.To(10),
					SyncPeriod:                 &v,
					RespectSyncPeriodOverwrite: ptr.To(true),
					ReconcileInMaintenanceOnly: ptr.To(true),
					RetryDuration:              &v,
					DNSEntryTTLSeconds:         ptr.To[int64](60),
				},
			}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.Shoot.ConcurrentSyncs).To(PointTo(Equal(10)))
			Expect(obj.Controllers.Shoot.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: 2 * time.Hour})))
			Expect(obj.Controllers.Shoot.RespectSyncPeriodOverwrite).To(PointTo(Equal(true)))
			Expect(obj.Controllers.Shoot.ReconcileInMaintenanceOnly).To(PointTo(Equal(true)))
			Expect(obj.Controllers.Shoot.RetryDuration).To(PointTo(Equal(metav1.Duration{Duration: 2 * time.Hour})))
			Expect(obj.Controllers.Shoot.DNSEntryTTLSeconds).To(PointTo(Equal(int64(60))))
		})
	})

	Describe("ShootCareControllerConfiguration defaulting", func() {
		It("should default the shoot care controller configuration", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.ShootCare.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: time.Minute})))
			Expect(obj.Controllers.ShootCare.ConcurrentSyncs).To(PointTo(Equal(20)))
			Expect(obj.Controllers.ShootCare.StaleExtensionHealthChecks.Enabled).To(BeTrue())
			Expect(obj.Controllers.ShootCare.StaleExtensionHealthChecks.Threshold).To(PointTo(Equal(metav1.Duration{Duration: 5 * time.Minute})))
		})

		It("should not overwrite already set values for the shoot care controller configuration", func() {
			syncPeriod := metav1.Duration{Duration: 2 * time.Minute}
			obj.Controllers = &GardenletControllerConfiguration{
				ShootCare: &ShootCareControllerConfiguration{
					SyncPeriod:                 &syncPeriod,
					ConcurrentSyncs:            ptr.To(10),
					StaleExtensionHealthChecks: &StaleExtensionHealthChecks{Enabled: false},
				},
			}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.ShootCare.SyncPeriod).To(PointTo(Equal(syncPeriod)))
			Expect(obj.Controllers.ShootCare.ConcurrentSyncs).To(PointTo(Equal(10)))
			Expect(obj.Controllers.ShootCare.StaleExtensionHealthChecks.Enabled).To(BeFalse())
		})
	})

	Describe("StaleExtensionHealthChecks defaulting", func() {
		It("should default the stale extension health checks", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.ShootCare.StaleExtensionHealthChecks.Threshold).To(PointTo(Equal(metav1.Duration{Duration: 5 * time.Minute})))
		})

		It("should not overwrite already set values for the stale extension health checks", func() {
			threshold := metav1.Duration{Duration: 2 * time.Minute}
			obj.Controllers = &GardenletControllerConfiguration{
				ShootCare: &ShootCareControllerConfiguration{
					StaleExtensionHealthChecks: &StaleExtensionHealthChecks{Threshold: &threshold},
				},
			}

			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.ShootCare.StaleExtensionHealthChecks.Threshold).To(PointTo(Equal(threshold)))
		})
	})

	Describe("ShootStateControllerConfiguration defaulting", func() {
		It("should default the shoot state controller configuration", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.ShootState.ConcurrentSyncs).To(PointTo(Equal(5)))
			Expect(obj.Controllers.ShootState.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: 6 * time.Hour})))
		})

		It("should not overwrite already set values for the shoot state controller configuration", func() {
			syncPeriod := metav1.Duration{Duration: 2 * time.Hour}
			obj.Controllers = &GardenletControllerConfiguration{
				ShootState: &ShootStateControllerConfiguration{
					SyncPeriod:      &syncPeriod,
					ConcurrentSyncs: ptr.To(10),
				},
			}

			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.ShootState.ConcurrentSyncs).To(PointTo(Equal(10)))
			Expect(obj.Controllers.ShootState.SyncPeriod).To(PointTo(Equal(syncPeriod)))
		})
	})

	Describe("NetworkPolicyControllerConfiguration defaulting", func() {
		It("should default the network policy controller configuration", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.NetworkPolicy.ConcurrentSyncs).To(PointTo(Equal(5)))
		})

		It("should not overwrite already set values for the network policy controller configuration", func() {
			obj.Controllers = &GardenletControllerConfiguration{
				NetworkPolicy: &NetworkPolicyControllerConfiguration{ConcurrentSyncs: ptr.To(10)},
			}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.NetworkPolicy.ConcurrentSyncs).To(PointTo(Equal(10)))
		})
	})

	Describe("ManagedSeedControllerConfiguration defaulting", func() {
		It("should default the managed seed controller configuration", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.ManagedSeed.ConcurrentSyncs).To(PointTo(Equal(DefaultControllerConcurrentSyncs)))
			Expect(obj.Controllers.ManagedSeed.SyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: 1 * time.Hour})))
			Expect(obj.Controllers.ManagedSeed.WaitSyncPeriod).To(PointTo(Equal(metav1.Duration{Duration: 15 * time.Second})))
			Expect(obj.Controllers.ManagedSeed.SyncJitterPeriod).To(PointTo(Equal(metav1.Duration{Duration: 5 * time.Minute})))
			Expect(obj.Controllers.ManagedSeed.JitterUpdates).To(PointTo(BeFalse()))
		})

		It("should not overwrite already set values for the managed seed controller configuration", func() {
			v := metav1.Duration{Duration: 2 * time.Minute}
			obj.Controllers = &GardenletControllerConfiguration{
				ManagedSeed: &ManagedSeedControllerConfiguration{
					ConcurrentSyncs:  ptr.To(10),
					SyncPeriod:       &v,
					WaitSyncPeriod:   &v,
					SyncJitterPeriod: &v,
					JitterUpdates:    ptr.To(true),
				},
			}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.ManagedSeed.ConcurrentSyncs).To(PointTo(Equal(10)))
			Expect(obj.Controllers.ManagedSeed.SyncPeriod).To(PointTo(Equal(v)))
			Expect(obj.Controllers.ManagedSeed.WaitSyncPeriod).To(PointTo(Equal(v)))
			Expect(obj.Controllers.ManagedSeed.SyncJitterPeriod).To(PointTo(Equal(v)))
			Expect(obj.Controllers.ManagedSeed.JitterUpdates).To(PointTo(BeTrue()))
		})
	})

	Describe("TokenRequestorServiceAccountControllerConfiguration defaulting", func() {
		It("should default the token requestor service account controller configuration", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.TokenRequestorServiceAccount.ConcurrentSyncs).To(PointTo(Equal(5)))
		})

		It("should not overwrite already set values for the token requestor controller configuration", func() {
			obj.Controllers = &GardenletControllerConfiguration{
				TokenRequestorServiceAccount: &TokenRequestorServiceAccountControllerConfiguration{ConcurrentSyncs: ptr.To(10)},
			}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.TokenRequestorServiceAccount.ConcurrentSyncs).To(PointTo(Equal(10)))
		})
	})

	Describe("TokenRequestorWorkloadIdentityControllerConfiguration defaulting", func() {
		It("should default the token requestor workload identity controller configuration", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.TokenRequestorWorkloadIdentity.ConcurrentSyncs).To(PointTo(Equal(5)))
		})

		It("should not overwrite already set values for the token requestor controller configuration", func() {
			obj.Controllers = &GardenletControllerConfiguration{
				TokenRequestorWorkloadIdentity: &TokenRequestorWorkloadIdentityControllerConfiguration{ConcurrentSyncs: ptr.To(10)},
			}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.TokenRequestorWorkloadIdentity.ConcurrentSyncs).To(PointTo(Equal(10)))
		})
	})

	Describe("VPAEvictionRequirementsControllerConfiguration defaulting", func() {
		It("should default the VPA eviction requirements controller configuration", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.VPAEvictionRequirements.ConcurrentSyncs).To(PointTo(Equal(5)))
		})

		It("should not overwrite already set values for the VPA eviction requirements controller configuration", func() {
			obj.Controllers = &GardenletControllerConfiguration{
				VPAEvictionRequirements: &VPAEvictionRequirementsControllerConfiguration{ConcurrentSyncs: ptr.To(10)},
			}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Controllers.VPAEvictionRequirements.ConcurrentSyncs).To(PointTo(Equal(10)))
		})
	})

	Describe("LeaderElectionConfiguration defaulting", func() {
		It("should correctly default the leader election configuration", func() {
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

		It("should not overwrite already set values for the leader election configuration", func() {
			expectedLeaderElection := &componentbaseconfigv1alpha1.LeaderElectionConfiguration{
				LeaderElect:       ptr.To(true),
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

	Describe("ServerConfiguration defaulting", func() {
		It("should default the HTTP server configuration", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Server.HealthProbes.BindAddress).To(BeEmpty())
			Expect(obj.Server.HealthProbes.Port).To(Equal(2728))
			Expect(obj.Server.Metrics.BindAddress).To(BeEmpty())
			Expect(obj.Server.Metrics.Port).To(Equal(2729))
		})

		It("should not overwrite already set values for the HTTP server configuration", func() {
			obj.Server = ServerConfiguration{
				HealthProbes: &Server{
					BindAddress: "127.0.0.0",
					Port:        1010,
				},
				Metrics: &Server{
					BindAddress: "127.0.0.1",
					Port:        1011,
				},
			}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Server.HealthProbes.BindAddress).To(Equal("127.0.0.0"))
			Expect(obj.Server.HealthProbes.Port).To(Equal(1010))
			Expect(obj.Server.Metrics.BindAddress).To(Equal("127.0.0.1"))
			Expect(obj.Server.Metrics.Port).To(Equal(1011))
		})
	})

	Describe("Logging defaulting", func() {
		It("should default the logging configuration", func() {
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

		It("should not overwrite already set values for the logging configuration", func() {
			gardenValiStorage := resource.MustParse("10Gi")
			expectedLogging := &Logging{
				Enabled: ptr.To(true),
				Vali: &Vali{
					Enabled: ptr.To(false),
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
					Enabled: ptr.To(false),
				},
			}

			obj.Logging = expectedLogging.DeepCopy()
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Logging).To(Equal(expectedLogging))
		})
	})

	Describe("SNI defaulting", func() {
		It("should default the SNI", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.SNI.Ingress).NotTo(BeNil())
		})
	})

	Describe("SNIIngress defaulting", func() {
		It("should default the SNI ingressgateway", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.SNI.Ingress.Namespace).To(PointTo(Equal("istio-ingress")))
			Expect(obj.SNI.Ingress.ServiceName).To(PointTo(Equal("istio-ingressgateway")))
			Expect(obj.SNI.Ingress.Labels).To(Equal(map[string]string{
				"app":   "istio-ingressgateway",
				"istio": "ingressgateway",
			}))
		})

		It("should not overwrite already set values for the SNI ingressgateway", func() {
			obj.SNI = &SNI{
				Ingress: &SNIIngress{
					Namespace:   ptr.To("namespace"),
					ServiceName: ptr.To("svc"),
					Labels:      map[string]string{"label1": "value1"},
				},
			}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.SNI.Ingress.Namespace).To(PointTo(Equal("namespace")))
			Expect(obj.SNI.Ingress.ServiceName).To(PointTo(Equal("svc")))
			Expect(obj.SNI.Ingress.Labels).To(Equal(map[string]string{"label1": "value1"}))
		})
	})

	Describe("ETCDConfig defaulting", func() {
		It("should correctly default ETCD configuration", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.ETCDConfig).NotTo(BeNil())
			Expect(obj.ETCDConfig.ETCDController).NotTo(BeNil())
			Expect(obj.ETCDConfig.CustodianController).NotTo(BeNil())
			Expect(obj.ETCDConfig.BackupCompactionController).NotTo(BeNil())
		})
	})

	Describe("ETCDController defaulting", func() {
		It("should default the ETCD controller", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.ETCDConfig.ETCDController.Workers).To(PointTo(Equal(int64(50))))
		})

		It("should not overwrite already set values for the ETCD controller", func() {
			obj.ETCDConfig = &ETCDConfig{
				ETCDController: &ETCDController{Workers: ptr.To[int64](5)},
			}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.ETCDConfig.ETCDController.Workers).To(PointTo(Equal(int64(5))))
		})
	})

	Describe("CustodianController defaulting", func() {
		It("should default the ETCD custodian controller", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.ETCDConfig.CustodianController.Workers).To(PointTo(Equal(int64(10))))
		})

		It("should not overwrite already set values for the ETCD custodian controller", func() {
			obj.ETCDConfig = &ETCDConfig{
				CustodianController: &CustodianController{Workers: ptr.To[int64](5)},
			}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.ETCDConfig.CustodianController.Workers).To(PointTo(Equal(int64(5))))
		})
	})

	Describe("BackupCompactionController defaulting", func() {
		It("should default the ETCD backup compaction controller", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.ETCDConfig.BackupCompactionController.Workers).To(PointTo(Equal(int64(3))))
			Expect(obj.ETCDConfig.BackupCompactionController.EnableBackupCompaction).To(PointTo(Equal(false)))
			Expect(obj.ETCDConfig.BackupCompactionController.EventsThreshold).To(PointTo(Equal(int64(1000000))))
			Expect(obj.ETCDConfig.BackupCompactionController.MetricsScrapeWaitDuration).To(PointTo(Equal(metav1.Duration{Duration: 60 * time.Second})))
		})

		It("should not overwrite already set values for the ETCD backup compaction controller", func() {
			v := metav1.Duration{Duration: 30 * time.Second}
			obj.ETCDConfig = &ETCDConfig{
				BackupCompactionController: &BackupCompactionController{
					Workers:                   ptr.To[int64](4),
					EnableBackupCompaction:    ptr.To(true),
					EventsThreshold:           ptr.To[int64](900000),
					MetricsScrapeWaitDuration: &v,
				}}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.ETCDConfig.BackupCompactionController.Workers).To(PointTo(Equal(int64(4))))
			Expect(obj.ETCDConfig.BackupCompactionController.EnableBackupCompaction).To(PointTo(Equal(true)))
			Expect(obj.ETCDConfig.BackupCompactionController.EventsThreshold).To(PointTo(Equal(int64(900000))))
			Expect(obj.ETCDConfig.BackupCompactionController.MetricsScrapeWaitDuration).To(PointTo(Equal(v)))
		})
	})

	Describe("ExposureClassHandler defaulting", func() {
		It("should default the gardenlets exposure class handlers sni config", func() {
			obj.ExposureClassHandlers = []ExposureClassHandler{
				{Name: "test1"},
				{Name: "test2", SNI: &SNI{}},
			}

			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.ExposureClassHandlers[0].SNI).ToNot(BeNil())
			Expect(obj.ExposureClassHandlers[0].SNI.Ingress).ToNot(BeNil())
			Expect(obj.ExposureClassHandlers[0].SNI.Ingress.Namespace).ToNot(BeNil())
			Expect(*obj.ExposureClassHandlers[0].SNI.Ingress.Namespace).To(Equal("istio-ingress-handler-" + obj.ExposureClassHandlers[0].Name))
			Expect(*obj.ExposureClassHandlers[0].SNI.Ingress.ServiceName).To(Equal("istio-ingressgateway"))
			Expect(obj.ExposureClassHandlers[0].SNI.Ingress.Labels).To(Equal(map[string]string{
				"app":                 "istio-ingressgateway",
				"gardener.cloud/role": "exposureclass-handler",
			}))

			Expect(obj.ExposureClassHandlers[1].SNI.Ingress).ToNot(BeNil())
			Expect(obj.ExposureClassHandlers[1].SNI.Ingress.Namespace).ToNot(BeNil())
			Expect(*obj.ExposureClassHandlers[1].SNI.Ingress.Namespace).To(Equal("istio-ingress-handler-" + obj.ExposureClassHandlers[1].Name))
			Expect(*obj.ExposureClassHandlers[1].SNI.Ingress.ServiceName).To(Equal("istio-ingressgateway"))
			Expect(obj.ExposureClassHandlers[1].SNI.Ingress.Labels).To(Equal(map[string]string{
				"app":                 "istio-ingressgateway",
				"gardener.cloud/role": "exposureclass-handler",
			}))
		})

		It("should not overwrite already set values for the gardenlets exposure class handlers sni config", func() {
			obj.ExposureClassHandlers = []ExposureClassHandler{
				{Name: "test1"},
				{Name: "test2", SNI: &SNI{
					Ingress: &SNIIngress{
						Namespace:   ptr.To("namespace"),
						ServiceName: ptr.To("svc"),
						Labels:      map[string]string{"label1": "value1"},
					},
				}},
			}

			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.ExposureClassHandlers[0].SNI).ToNot(BeNil())
			Expect(obj.ExposureClassHandlers[0].SNI.Ingress).ToNot(BeNil())
			Expect(obj.ExposureClassHandlers[0].SNI.Ingress.Namespace).ToNot(BeNil())
			Expect(*obj.ExposureClassHandlers[0].SNI.Ingress.Namespace).To(Equal("istio-ingress-handler-" + obj.ExposureClassHandlers[0].Name))
			Expect(*obj.ExposureClassHandlers[0].SNI.Ingress.ServiceName).To(Equal("istio-ingressgateway"))
			Expect(obj.ExposureClassHandlers[0].SNI.Ingress.Labels).To(Equal(map[string]string{
				"app":                 "istio-ingressgateway",
				"gardener.cloud/role": "exposureclass-handler",
			}))

			Expect(obj.ExposureClassHandlers[1].SNI.Ingress).ToNot(BeNil())
			Expect(obj.ExposureClassHandlers[1].SNI.Ingress.Namespace).ToNot(BeNil())
			Expect(*obj.ExposureClassHandlers[1].SNI.Ingress.Namespace).To(Equal("namespace"))
			Expect(*obj.ExposureClassHandlers[1].SNI.Ingress.ServiceName).To(Equal("svc"))
			Expect(obj.ExposureClassHandlers[1].SNI.Ingress.Labels).To(Equal(map[string]string{"label1": "value1"}))
		})
	})

	Describe("MonitoringConfig defaulting", func() {
		It("should default the monitoring configuration", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Monitoring.Shoot).ToNot(BeNil())
		})
	})

	Describe("ShootMonitoringConfig defaulting", func() {
		It("should default the shoot monitoring configuration", func() {
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Monitoring.Shoot).ToNot(BeNil())
			Expect(*obj.Monitoring.Shoot.Enabled).To(BeTrue())
		})

		It("should not overwrite already set values for the shoot monitoring configuration", func() {
			obj.Monitoring = &MonitoringConfig{
				&ShootMonitoringConfig{
					Enabled: ptr.To(false),
				}}
			SetObjectDefaults_GardenletConfiguration(obj)

			Expect(obj.Monitoring.Shoot).ToNot(BeNil())
			Expect(*obj.Monitoring.Shoot.Enabled).To(BeFalse())
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
