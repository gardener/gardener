// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	. "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/klog"
	"k8s.io/utils/pointer"
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
			Expect(obj.Controllers.ShootStateSync).NotTo(BeNil())
			Expect(obj.Controllers.ManagedSeed).NotTo(BeNil())
			Expect(obj.LeaderElection).NotTo(BeNil())
			Expect(obj.LogLevel).To(PointTo(Equal(logger.InfoLevel)))
			Expect(obj.LogFormat).To(PointTo(Equal(logger.FormatJSON)))
			Expect(obj.KubernetesLogLevel).To(PointTo(Equal(klog.Level(0))))
			Expect(obj.Server.HTTPS.BindAddress).To(Equal("0.0.0.0"))
			Expect(obj.Server.HTTPS.Port).To(Equal(2720))
			Expect(obj.SNI).ToNot(BeNil())
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
				Expect(obj.LeaderElection.LockObjectNamespace).To(PointTo(Equal("garden")))
				Expect(obj.LeaderElection.LockObjectName).To(PointTo(Equal("gardenlet-leader-election")))
			})
			It("should not overwrite custom settings", func() {
				expectedLeaderElection := &LeaderElectionConfiguration{
					LeaderElectionConfiguration: componentbaseconfigv1alpha1.LeaderElectionConfiguration{
						LeaderElect:   pointer.Bool(true),
						ResourceLock:  "foo",
						RetryPeriod:   metav1.Duration{Duration: 40 * time.Second},
						RenewDeadline: metav1.Duration{Duration: 41 * time.Second},
						LeaseDuration: metav1.Duration{Duration: 42 * time.Second},
					},
					LockObjectName:      pointer.String("lock-object"),
					LockObjectNamespace: pointer.String("other-garden-ns"),
				}
				obj.LeaderElection = expectedLeaderElection.DeepCopy()
				SetObjectDefaults_GardenletConfiguration(obj)

				Expect(obj.LeaderElection).To(Equal(expectedLeaderElection))
			})
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
})
